package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/audit"
	"oemrpa/internal/redis"
	"oemrpa/internal/repository"
	"oemrpa/internal/site"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"
)

// PC 扫码登录：PC 渲染公众号网页授权链接的二维码，用户用微信扫一扫在微信内置浏览器中完成授权，
// PC 轮询会话状态取得登录态。手机侧只授权、不签发任何登录票据。
const (
	wechatModeQR = "qr"

	wechatQRModeLogin = "login" // 扫码登录
	wechatQRModeBind  = "bind"  // 已登录用户扫码绑定

	wechatQRPrefix     = "wechat:qr:"
	wechatQRPollPrefix = "wechat:qr_poll:"

	wechatQRSessionTTL  = 5 * time.Minute
	wechatQRNeedBindTTL = 10 * time.Minute
	wechatQRPollWindow  = time.Minute
	wechatQRPollMax     = 60 // 单会话每分钟轮询上限（2 秒一次约 30 次）

	wechatQRPollInterval = 2 // 前端轮询间隔（秒）
)

// 扫码会话状态
const (
	wechatQRStatusPending    = "pending"
	wechatQRStatusNeedBind   = "need_bind"
	wechatQRStatusAuthorized = "authorized"
	wechatQRStatusBound      = "bound"
	wechatQRStatusConflict   = "conflict"
)

var (
	errWechatQRNotConfigured = errors.New("微信登录未配置")
	errWechatQRStorage       = errors.New("微信登录初始化失败")
)

// wechatQRSession PC 扫码会话（存 Redis，PC 凭 qr_ticket 轮询；qr_ticket 只存在于 PC 内存，不进二维码）
type wechatQRSession struct {
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	BindUserID  int64  `json:"bind_user_id"` // bind 模式：发起绑定的登录用户
	UserID      int64  `json:"user_id"`
	OpenID      string `json:"openid"`
	UnionID     string `json:"unionid"`
	BindTicket  string `json:"bind_ticket"`  // need_bind 时供 PC 提交手机号绑定的票据
	LoginTicket string `json:"login_ticket"` // authorized 时供 PC 换取 JWT 的一次性票据
}

// WechatQRSession 创建 PC 扫码登录会话，返回二维码内容（公众号授权链接）与轮询凭据
func (h *WechatLoginHandler) WechatQRSession(c *gin.Context) {
	qrTicket, qrURL, err := h.createQRSession(wechatQRModeLogin, 0)
	if err != nil {
		h.replyQRSessionError(c, err)
		return
	}
	audit.Log("wechat_qr_create", audit.KV("mode", wechatQRModeLogin), audit.KV("scene", upstream.SceneMP))
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"qr_ticket":     qrTicket,
			"qr_url":        qrURL,
			"expires_in":    int(wechatQRSessionTTL.Seconds()),
			"poll_interval": wechatQRPollInterval,
		},
	})
}

// WechatQRBindSession 创建 PC 扫码绑定会话（需登录，二维码扫到后把该微信绑到当前账号）
func (h *WechatLoginHandler) WechatQRBindSession(c *gin.Context) {
	bindUserID := c.GetInt64("user_id")
	qrTicket, qrURL, err := h.createQRSession(wechatQRModeBind, bindUserID)
	if err != nil {
		h.replyQRSessionError(c, err)
		return
	}
	audit.Log("wechat_qr_create", audit.KV("mode", wechatQRModeBind), audit.KV("scene", upstream.SceneMP),
		audit.KV("user_id", bindUserID))
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"qr_ticket":     qrTicket,
			"qr_url":        qrURL,
			"expires_in":    int(wechatQRSessionTTL.Seconds()),
			"poll_interval": wechatQRPollInterval,
		},
	})
}

// WechatQRPoll PC 轮询扫码会话状态：业务状态一律 code=0 返回，避免前端把「过期/待绑定」当异常弹窗
func (h *WechatLoginHandler) WechatQRPoll(c *gin.Context) {
	var req struct {
		QRTicket string `json:"qr_ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}
	if !h.pollThrottle(req.QRTicket) {
		c.JSON(http.StatusTooManyRequests, gin.H{"code": 429, "message": "请求过于频繁，请稍后再试", "retry_after": 60})
		return
	}

	s, err := h.loadQRSession(req.QRTicket)
	if err != nil {
		// 会话不存在或已过期
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"status": "expired", "expires_in": 0}})
		return
	}

	key := wechatQRPrefix + req.QRTicket
	ttl, _ := redis.TTL(key)
	if ttl <= 0 {
		ttl = wechatQRSessionTTL
	}

	// authorized 时惰性生成一次性登录票据（同一会话重复轮询返回同一票据）
	if s.Status == wechatQRStatusAuthorized && s.LoginTicket == "" {
		if ticket, terr := h.issueLoginTicket(s.UserID); terr == nil {
			s.LoginTicket = ticket
			_ = h.saveQRSession(req.QRTicket, s, ttl)
		}
	}

	data := gin.H{"status": s.Status, "expires_in": int(ttl.Seconds())}
	if s.Status == wechatQRStatusAuthorized && s.LoginTicket != "" {
		data["login_ticket"] = s.LoginTicket
	}
	if s.Status == wechatQRStatusNeedBind && s.BindTicket != "" {
		data["bind_ticket"] = s.BindTicket
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}

// createQRSession 生成扫码会话：state（含会话指针）写 Redis，二维码内容即公众号授权链接
func (h *WechatLoginHandler) createQRSession(mode string, bindUserID int64) (string, string, error) {
	cli := h.rt.WechatOAuth()
	if cli == nil || !cli.Available() {
		return "", "", errWechatQRNotConfigured
	}

	qrTicket := utils.GenerateRandomKey(24)
	state := utils.GenerateRandomKey(16)

	statePayload, _ := json.Marshal(wechatState{Scene: upstream.SceneMP, Mode: wechatModeQR, QRSession: qrTicket})
	if err := redis.Set(wechatStatePrefix+state, string(statePayload), wechatStateTTL); err != nil {
		return "", "", errWechatQRStorage
	}
	if err := h.saveQRSession(qrTicket, &wechatQRSession{
		Mode: mode, Status: wechatQRStatusPending, BindUserID: bindUserID,
	}, wechatQRSessionTTL); err != nil {
		return "", "", errWechatQRStorage
	}

	qrURL, err := cli.AuthorizeURL(site.Platform().ConsoleBase()+wechatCallbackPath, state)
	if err != nil {
		return "", "", errWechatQRNotConfigured
	}
	return qrTicket, qrURL, nil
}

// handleQRCallback 扫码授权回跳：写会话状态并 302 到手机侧纯展示页（不下发任何票据）
func (h *WechatLoginHandler) handleQRCallback(c *gin.Context, st *wechatState) {
	consoleBase := site.Platform().ConsoleBase()

	s, err := h.loadQRSession(st.QRSession)
	if err != nil {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=expired")
		return
	}
	// 终态后拒绝二次回调，防身份被覆盖
	if s.Status != wechatQRStatusPending {
		audit.Log("wechat_qr_used", audit.KV("mode", s.Mode))
		c.Redirect(http.StatusFound, consoleBase+"/wechat/scan?r=used")
		return
	}

	cli := h.rt.WechatOAuth()
	code := c.Query("code")
	if code == "" || cli == nil || !cli.Available() {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
		return
	}
	wu, err := cli.ExchangeCode(code)
	if err != nil {
		log.Printf("微信扫码换取用户标识失败: %v", err)
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
		return
	}
	s.OpenID, s.UnionID = wu.OpenID, wu.UnionID

	result := "ok"
	ttl := wechatQRSessionTTL

	if s.Mode == wechatQRModeBind {
		if berr := h.bindWechat(s.BindUserID, wu); berr != nil {
			if !errors.Is(berr, repository.ErrWechatAlreadyBound) {
				log.Printf("微信扫码绑定失败 [user_id=%d]: %v", s.BindUserID, berr)
				c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
				return
			}
			s.Status, result = wechatQRStatusConflict, "conflict"
		} else {
			s.Status, result = wechatQRStatusBound, "bound"
			audit.Log("wechat_bind", audit.KV("user_id", s.BindUserID), audit.KV("scene", upstream.SceneMP),
				audit.KV("openid", wu.OpenID), audit.KV("unionid", wu.UnionID), audit.KV("method", "qr"))
		}
	} else {
		bound, uerr := h.userRepo.GetUserByWechat(wu.UnionID, wu.OpenID)
		if uerr != nil && !errors.Is(uerr, repository.ErrUserNotFound) {
			c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
			return
		}
		if bound != nil {
			s.Status, s.UserID = wechatQRStatusAuthorized, bound.ID
			h.backfillWechatMP(bound.ID, wu)
			audit.Log("wechat_oauth", audit.KV("scene", wu.Scene), audit.KV("openid", wu.OpenID),
				audit.KV("unionid", wu.UnionID), audit.KV("matched", 1), audit.KV("user_id", bound.ID), audit.KV("mode", wechatModeQR))
		} else {
			bindTicket, berr := h.issueBindTicket(wu, st.QRSession)
			if berr != nil {
				c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
				return
			}
			s.Status, s.BindTicket, ttl, result = wechatQRStatusNeedBind, bindTicket, wechatQRNeedBindTTL, "need_bind"
			audit.Log("wechat_oauth", audit.KV("scene", wu.Scene), audit.KV("openid", wu.OpenID),
				audit.KV("unionid", wu.UnionID), audit.KV("matched", 0), audit.KV("mode", wechatModeQR))
		}
	}

	_ = h.saveQRSession(st.QRSession, s, ttl)
	audit.Log("wechat_qr_scanned", audit.KV("mode", s.Mode), audit.KV("openid", wu.OpenID), audit.KV("unionid", wu.UnionID))
	c.Redirect(http.StatusFound, consoleBase+"/wechat/scan?r="+result)
}

// markQRSessionAuthorized 回写扫码会话为已授权（PC 侧绑定完成后）
func (h *WechatLoginHandler) markQRSessionAuthorized(qrTicket string, userID int64) {
	s, err := h.loadQRSession(qrTicket)
	if err != nil {
		return
	}
	s.Status, s.UserID = wechatQRStatusAuthorized, userID
	ttl, _ := redis.TTL(wechatQRPrefix + qrTicket)
	if ttl <= 0 {
		ttl = wechatQRSessionTTL
	}
	_ = h.saveQRSession(qrTicket, s, ttl)
}

// loadQRSession 读取扫码会话
func (h *WechatLoginHandler) loadQRSession(qrTicket string) (*wechatQRSession, error) {
	raw, err := redis.Get(wechatQRPrefix + qrTicket)
	if err != nil {
		return nil, err
	}
	var s wechatQRSession
	if json.Unmarshal([]byte(raw), &s) != nil {
		return nil, errors.New("扫码会话数据无效")
	}
	return &s, nil
}

// saveQRSession 写入扫码会话
func (h *WechatLoginHandler) saveQRSession(qrTicket string, s *wechatQRSession, ttl time.Duration) error {
	payload, _ := json.Marshal(s)
	return redis.Set(wechatQRPrefix+qrTicket, string(payload), ttl)
}

// pollThrottle 单会话轮询节流：Redis 异常时拒绝（与限流中间件一致的 fail-closed）
func (h *WechatLoginHandler) pollThrottle(qrTicket string) bool {
	key := wechatQRPollPrefix + qrTicket
	count, err := redis.Incr(key)
	if err != nil {
		log.Printf("扫码轮询限流 Redis 异常，拒绝请求: %v", err)
		return false
	}
	if count == 1 {
		_ = redis.Expire(key, wechatQRPollWindow)
	}
	return count <= wechatQRPollMax
}

// replyQRSessionError 扫码会话创建失败的统一错误响应
func (h *WechatLoginHandler) replyQRSessionError(c *gin.Context, err error) {
	msg := "微信登录初始化失败"
	if errors.Is(err, errWechatQRNotConfigured) {
		msg = "微信登录未配置"
	}
	c.JSON(http.StatusOK, gin.H{"code": 500, "message": msg})
}
