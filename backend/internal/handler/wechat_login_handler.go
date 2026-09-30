package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/audit"
	"oemrpa/internal/model"
	"oemrpa/internal/redis"
	"oemrpa/internal/repository"
	"oemrpa/internal/runtime"
	"oemrpa/internal/service"
	"oemrpa/internal/site"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"
)

// 微信登录回调路径（硬编码，域名按品牌主域推导）：与前端中转页路径 /wechat/callback 刻意不同，
// 前端该路径属 SPA 路由（/console/* 会被 Nginx 反代到后端，故不能复用）。
const wechatCallbackPath = "/console/wechat/callback"

// wechatQueryScenePC 仅用于识别历史前端可能传入的 scene=pc（PC 端已改为扫码登录，页面内授权不支持）
const wechatQueryScenePC = "pc"

// Redis 键前缀与有效期：state 用于防 CSRF；ticket / bind_ticket 均为一次性凭据
const (
	wechatStatePrefix      = "wechat:state:"
	wechatTicketPrefix     = "wechat:ticket:"
	wechatBindTicketPrefix = "wechat:bind_ticket:"

	wechatStateTTL      = 10 * time.Minute
	wechatTicketTTL     = 2 * time.Minute
	wechatBindTicketTTL = 10 * time.Minute
)

// wechatState 授权前的会话状态（存 Redis，回调时一次性消费）
type wechatState struct {
	Scene      string `json:"scene"`
	Mode       string `json:"mode,omitempty"`       // 空=手机端登录/设置页绑定；qr=PC 扫码会话
	BindUserID int64  `json:"bind_user_id"`         // >0 表示已登录用户在账户设置页发起绑定
	QRSession  string `json:"qr_session,omitempty"` // mode=qr 时指向扫码会话（wechat:qr:<qr_ticket>）
}

// wechatTicket 一次性登录票据（前端凭它换 JWT，避免 token 出现在 URL/日志）
type wechatTicket struct {
	UserID int64 `json:"user_id"`
}

// wechatBindTicket 未绑定时的一次性绑定票据（携带本次授权取得的微信标识）
type wechatBindTicket struct {
	Scene     string `json:"scene"`
	OpenID    string `json:"openid"`
	UnionID   string `json:"unionid"`
	Nickname  string `json:"nickname"`
	QRSession string `json:"qr_session,omitempty"` // 非空表示 PC 扫码会话发起，绑定成功后回写该会话
}

// WechatLoginHandler 微信一键登录（公众号网页授权：手机端页面内授权 / PC 扫码授权）
type WechatLoginHandler struct {
	rt           *runtime.Runtime
	userRepo     *repository.UserRepository
	userService  *service.UserService
	jwtManager   *utils.JWTManager
	loginLogRepo *repository.LoginLogRepository
}

func NewWechatLoginHandler(
	rt *runtime.Runtime,
	userRepo *repository.UserRepository,
	userService *service.UserService,
	jwtManager *utils.JWTManager,
	loginLogRepo *repository.LoginLogRepository,
) *WechatLoginHandler {
	return &WechatLoginHandler{
		rt:           rt,
		userRepo:     userRepo,
		userService:  userService,
		jwtManager:   jwtManager,
		loginLogRepo: loginLogRepo,
	}
}

// WechatAuthorize 发起微信登录：生成一次性 state 后返回公众号授权地址（仅在微信内置浏览器内打开有效）。
func (h *WechatLoginHandler) WechatAuthorize(c *gin.Context) {
	if c.Query("scene") == wechatQueryScenePC {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "PC 端请使用扫码登录"})
		return
	}
	h.authorize(c, 0)
}

// WechatBindAuthorize 已登录用户发起微信绑定：state 记录 userID，回调时直接写绑定（不签发登录态）。
func (h *WechatLoginHandler) WechatBindAuthorize(c *gin.Context) {
	if c.Query("scene") == wechatQueryScenePC {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "PC 端请在绑定弹层中扫码"})
		return
	}
	h.authorize(c, c.GetInt64("user_id"))
}

// WechatCallback 微信授权回跳：校验 state 后换取微信标识。
// mode=qr 分流到扫码会话（手机侧只授权、不签发登录态）；其余按原逻辑 302 到前端中转页。
func (h *WechatLoginHandler) WechatCallback(c *gin.Context) {
	consoleBase := site.Platform().ConsoleBase()

	stateKey := wechatStatePrefix + c.Query("state")
	raw, err := redis.Get(stateKey)
	if err != nil {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=expired")
		return
	}
	// 一次性消费，防重放/CSRF
	_ = redis.Del(stateKey)

	var st wechatState
	if json.Unmarshal([]byte(raw), &st) != nil {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=expired")
		return
	}

	if st.Mode == wechatModeQR {
		h.handleQRCallback(c, &st)
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
		log.Printf("微信登录换取用户标识失败: %v", err)
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
		return
	}

	// 已登录用户在账户设置页发起：直接写绑定，不签发登录态
	if st.BindUserID > 0 {
		if berr := h.bindWechat(st.BindUserID, wu); berr != nil {
			c.Redirect(http.StatusFound, consoleBase+"/settings?wechat=conflict")
			return
		}
		audit.Log("wechat_bind", audit.KV("user_id", st.BindUserID), audit.KV("scene", wu.Scene),
			audit.KV("openid", wu.OpenID), audit.KV("unionid", wu.UnionID), audit.KV("method", "settings"))
		c.Redirect(http.StatusFound, consoleBase+"/settings?wechat=bound")
		return
	}

	// 命中已绑定账号：发一次性登录票据，由前端换取 JWT
	bound, err := h.userRepo.GetUserByWechat(wu.UnionID, wu.OpenID)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
		return
	}
	if bound != nil {
		ticket, terr := h.issueLoginTicket(bound.ID)
		if terr != nil {
			c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
			return
		}
		audit.Log("wechat_oauth", audit.KV("scene", wu.Scene), audit.KV("openid", wu.OpenID),
			audit.KV("unionid", wu.UnionID), audit.KV("matched", 1), audit.KV("user_id", bound.ID))
		c.Redirect(http.StatusFound, consoleBase+"/wechat/callback?ticket="+ticket)
		return
	}

	// 未绑定：发一次性绑定票据，前端收集手机号与短信验证码后完成绑定登录
	bindTicket, berr := h.issueBindTicket(wu, "")
	if berr != nil {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
		return
	}
	audit.Log("wechat_oauth", audit.KV("scene", wu.Scene), audit.KV("openid", wu.OpenID),
		audit.KV("unionid", wu.UnionID), audit.KV("matched", 0))
	c.Redirect(http.StatusFound, consoleBase+"/wechat/bind?ticket="+bindTicket)
}

// WechatTicket 用一次性登录票据换取登录态（返回体与密码/短信登录一致）
func (h *WechatLoginHandler) WechatTicket(c *gin.Context) {
	var req struct {
		Ticket string `json:"ticket" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}

	ticketKey := wechatTicketPrefix + req.Ticket
	raw, err := redis.Get(ticketKey)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "登录票据已过期，请重新发起微信登录"})
		return
	}
	_ = redis.Del(ticketKey)

	var t wechatTicket
	if json.Unmarshal([]byte(raw), &t) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "登录票据无效，请重新发起微信登录"})
		return
	}

	user, err := h.userService.GetUserByID(t.UserID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "账号不存在"})
		return
	}
	if user.Status == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 403, "message": service.ErrUserDisabled.Error()})
		return
	}
	h.issueWechatLogin(c, user)
}

// WechatBind 未绑定时绑定已有账号：校验手机号与短信验证码后写入绑定并签发登录态。
// 票据携带 qr_session 时（PC 扫码会话发起）绑定成功后同步回写扫码会话。
func (h *WechatLoginHandler) WechatBind(c *gin.Context) {
	var req struct {
		BindTicket string `json:"bind_ticket" binding:"required"`
		Phone      string `json:"phone" binding:"required"`
		SMSCode    string `json:"sms_code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}

	ticketKey := wechatBindTicketPrefix + req.BindTicket
	raw, err := redis.Get(ticketKey)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "绑定已过期，请重新发起微信登录"})
		return
	}
	var bt wechatBindTicket
	if json.Unmarshal([]byte(raw), &bt) != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "绑定信息无效，请重新发起微信登录"})
		return
	}

	if err := validator.ValidatePhone(req.Phone); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请输入正确的手机号"})
		return
	}

	// 校验短信验证码（与短信验证码登录同一通道）
	if valid, verr := h.rt.SMS().VerifyCode(req.Phone, req.SMSCode); verr != nil || !valid {
		c.JSON(http.StatusOK, gin.H{"code": 401, "message": "短信验证码错误或已过期"})
		return
	}

	user, err := h.userService.GetUserByPhone(req.Phone)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "该手机号未注册，请先注册账号后再绑定"})
		return
	}
	if user.Status == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 403, "message": service.ErrUserDisabled.Error()})
		return
	}

	wu := &upstream.WechatOAuthUser{OpenID: bt.OpenID, UnionID: bt.UnionID, Nickname: bt.Nickname, Scene: upstream.SceneMP}
	// 该微信已绑定到其它账号时直接拒绝（避免换绑导致原账号丢失登录方式）
	if other, oerr := h.userRepo.GetUserByWechat(wu.UnionID, wu.OpenID); oerr == nil && other != nil && other.ID != user.ID {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "该微信已绑定其它账号，请先在该账号解绑"})
		return
	}
	if berr := h.bindWechat(user.ID, wu); berr != nil {
		if errors.Is(berr, repository.ErrWechatAlreadyBound) {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "该微信已绑定其它账号，请先在该账号解绑"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信绑定失败"})
		return
	}
	_ = redis.Del(ticketKey)

	// PC 扫码会话：绑定成功后回写会话，供 PC 轮询取得登录态
	if bt.QRSession != "" {
		h.markQRSessionAuthorized(bt.QRSession, user.ID)
	}

	audit.Log("wechat_bind", audit.KV("user_id", user.ID), audit.KV("scene", wu.Scene),
		audit.KV("openid", wu.OpenID), audit.KV("unionid", wu.UnionID), audit.KV("method", "sms"))
	h.issueWechatLogin(c, user)
}

// WechatBinding 查询当前登录用户的微信绑定状态
func (h *WechatLoginHandler) WechatBinding(c *gin.Context) {
	_, mpOpenID, nickname, err := h.userRepo.GetWechatBinding(c.GetInt64("user_id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取微信绑定状态失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"mp_bound": mpOpenID != "",
			"nickname": nickname,
		},
	})
}

// WechatUnbind 解绑公众号微信
func (h *WechatLoginHandler) WechatUnbind(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if c.Query("scene") != upstream.SceneMP {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid scene"})
		return
	}
	if err := h.userRepo.UnbindWechatMP(userID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "解除微信绑定失败"})
		return
	}
	audit.Log("wechat_unbind", audit.KV("user_id", userID), audit.KV("scene", upstream.SceneMP))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// authorize 生成一次性 state 并返回公众号授权地址（bindUserID>0 表示设置页发起绑定）
func (h *WechatLoginHandler) authorize(c *gin.Context, bindUserID int64) {
	cli := h.rt.WechatOAuth()
	if cli == nil || !cli.Available() {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信登录未配置"})
		return
	}

	state := utils.GenerateRandomKey(16)
	payload, _ := json.Marshal(wechatState{Scene: upstream.SceneMP, BindUserID: bindUserID})
	if err := redis.Set(wechatStatePrefix+state, string(payload), wechatStateTTL); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信登录初始化失败"})
		return
	}

	authorizeURL, err := cli.AuthorizeURL(site.Platform().ConsoleBase()+wechatCallbackPath, state)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信登录未配置"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"authorize_url": authorizeURL}})
}

// issueLoginTicket 生成一次性登录票据（前端凭它 POST 换取 JWT）
func (h *WechatLoginHandler) issueLoginTicket(userID int64) (string, error) {
	ticket := utils.GenerateRandomKey(24)
	payload, _ := json.Marshal(wechatTicket{UserID: userID})
	if err := redis.Set(wechatTicketPrefix+ticket, string(payload), wechatTicketTTL); err != nil {
		return "", err
	}
	return ticket, nil
}

// issueBindTicket 生成一次性绑定票据（qrSession 非空时携带扫码会话指针）
func (h *WechatLoginHandler) issueBindTicket(u *upstream.WechatOAuthUser, qrSession string) (string, error) {
	bindTicket := utils.GenerateRandomKey(24)
	payload, _ := json.Marshal(wechatBindTicket{
		Scene: u.Scene, OpenID: u.OpenID, UnionID: u.UnionID, Nickname: u.Nickname, QRSession: qrSession,
	})
	if err := redis.Set(wechatBindTicketPrefix+bindTicket, string(payload), wechatBindTicketTTL); err != nil {
		return "", err
	}
	return bindTicket, nil
}

// bindWechat 写入公众号 openid（unionid/昵称有空值时保留已有值）
func (h *WechatLoginHandler) bindWechat(userID int64, u *upstream.WechatOAuthUser) error {
	return h.userRepo.BindWechatMP(userID, u.OpenID, u.UnionID, u.Nickname)
}

// backfillWechatMP 回填公众号 openid：历史仅绑定过其它端的账号（unionid 命中）扫码后可平滑补齐本端绑定。
// 回填失败仅记日志（不影响本次登录）。
func (h *WechatLoginHandler) backfillWechatMP(userID int64, u *upstream.WechatOAuthUser) {
	_, mpOpenID, _, err := h.userRepo.GetWechatBinding(userID)
	if err != nil || mpOpenID != "" {
		return
	}
	if berr := h.userRepo.BindWechatMP(userID, u.OpenID, u.UnionID, u.Nickname); berr != nil {
		log.Printf("微信扫码回填公众号绑定失败 [user_id=%d]: %v", userID, berr)
		return
	}
	audit.Log("wechat_bind", audit.KV("user_id", userID), audit.KV("scene", upstream.SceneMP),
		audit.KV("openid", u.OpenID), audit.KV("unionid", u.UnionID), audit.KV("method", "qr_backfill"))
}

// issueWechatLogin 写登录日志（login_type=wechat）并签发 JWT
func (h *WechatLoginHandler) issueWechatLogin(c *gin.Context, user *model.User) {
	h.loginLogRepo.InsertUserLoginLog(&model.UserLoginLog{
		UserID:    user.ID,
		Account:   user.Phone,
		LoginType: "wechat",
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Status:    1,
	})
	_ = h.userRepo.UpdateLastLoginTime(user.ID)

	token, err := h.jwtManager.GenerateToken(user.ID, user.Phone, "user", 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "failed to generate token"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"user_id":   user.ID,
			"phone":     user.Phone,
			"username":  user.Username,
			"token":     token,
			"expire_in": 86400,
		},
	})
}
