package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
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
	BindUserID int64  `json:"bind_user_id"` // >0 表示已登录用户在账户设置页发起绑定
}

// wechatTicket 一次性登录票据（前端凭它换 JWT，避免 token 出现在 URL/日志）
type wechatTicket struct {
	UserID int64 `json:"user_id"`
}

// wechatBindTicket 未绑定时的一次性绑定票据（携带本次授权取得的微信标识）
type wechatBindTicket struct {
	Scene    string `json:"scene"`
	OpenID   string `json:"openid"`
	UnionID  string `json:"unionid"`
	Nickname string `json:"nickname"`
}

// WechatLoginHandler 微信一键登录（手机端公众号网页授权 / PC 开放平台扫码）
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

// WechatAuthorize 发起微信登录：生成一次性 state 后返回微信授权地址（前端跳转）。
func (h *WechatLoginHandler) WechatAuthorize(c *gin.Context) {
	h.authorize(c, resolveWechatScene(c, c.Query("scene")), 0)
}

// WechatBindAuthorize 已登录用户发起微信绑定：state 记录 userID，回调时直接写绑定（不签发登录态）。
func (h *WechatLoginHandler) WechatBindAuthorize(c *gin.Context) {
	h.authorize(c, resolveWechatScene(c, c.Query("scene")), c.GetInt64("user_id"))
}

// WechatCallback 微信授权回跳：校验 state 后换取微信标识，按绑定情况 302 到前端中转页。
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

	cli := h.rt.WechatOAuth()
	code := c.Query("code")
	if code == "" || cli == nil || !cli.Available(st.Scene) {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
		return
	}
	wu, err := cli.ExchangeCode(st.Scene, code)
	if err != nil {
		log.Printf("微信登录换取用户标识失败 [scene=%s]: %v", st.Scene, err)
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
	unionID, mpOpenID, openOpenID := wechatLookupKeys(wu)
	bound, err := h.userRepo.GetUserByWechat(unionID, mpOpenID, openOpenID)
	if err != nil && !errors.Is(err, repository.ErrUserNotFound) {
		c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
		return
	}
	if bound != nil {
		ticket := utils.GenerateRandomKey(24)
		payload, _ := json.Marshal(wechatTicket{UserID: bound.ID})
		if serr := redis.Set(wechatTicketPrefix+ticket, string(payload), wechatTicketTTL); serr != nil {
			c.Redirect(http.StatusFound, consoleBase+"/login?wechat=failed")
			return
		}
		audit.Log("wechat_oauth", audit.KV("scene", wu.Scene), audit.KV("openid", wu.OpenID),
			audit.KV("unionid", wu.UnionID), audit.KV("matched", 1), audit.KV("user_id", bound.ID))
		c.Redirect(http.StatusFound, consoleBase+"/wechat/callback?ticket="+ticket)
		return
	}

	// 未绑定：发一次性绑定票据，前端收集手机号与短信验证码后完成绑定登录
	bindTicket := utils.GenerateRandomKey(24)
	payload, _ := json.Marshal(wechatBindTicket{Scene: wu.Scene, OpenID: wu.OpenID, UnionID: wu.UnionID, Nickname: wu.Nickname})
	if serr := redis.Set(wechatBindTicketPrefix+bindTicket, string(payload), wechatBindTicketTTL); serr != nil {
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

	wu := &upstream.WechatOAuthUser{OpenID: bt.OpenID, UnionID: bt.UnionID, Nickname: bt.Nickname, Scene: bt.Scene}
	// 该微信已绑定到其它账号时直接拒绝（避免换绑导致原账号丢失登录方式）
	unionID, mpOpenID, openOpenID := wechatLookupKeys(wu)
	if other, oerr := h.userRepo.GetUserByWechat(unionID, mpOpenID, openOpenID); oerr == nil && other != nil && other.ID != user.ID {
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

	audit.Log("wechat_bind", audit.KV("user_id", user.ID), audit.KV("scene", wu.Scene),
		audit.KV("openid", wu.OpenID), audit.KV("unionid", wu.UnionID), audit.KV("method", "sms"))
	h.issueWechatLogin(c, user)
}

// WechatBinding 查询当前登录用户的微信绑定状态
func (h *WechatLoginHandler) WechatBinding(c *gin.Context) {
	_, mpOpenID, openOpenID, nickname, err := h.userRepo.GetWechatBinding(c.GetInt64("user_id"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取微信绑定状态失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"mp_bound":   mpOpenID != "",
			"open_bound": openOpenID != "",
			"nickname":   nickname,
		},
	})
}

// WechatUnbind 解绑指定场景的微信（scene：mp-公众号 pc-开放平台）
func (h *WechatLoginHandler) WechatUnbind(c *gin.Context) {
	userID := c.GetInt64("user_id")
	scene := c.Query("scene")

	var err error
	switch scene {
	case upstream.SceneMP:
		err = h.userRepo.UnbindWechatMP(userID)
	case upstream.ScenePC:
		err = h.userRepo.UnbindWechatOpen(userID)
	default:
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid scene"})
		return
	}
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "解除微信绑定失败"})
		return
	}
	audit.Log("wechat_unbind", audit.KV("user_id", userID), audit.KV("scene", scene))
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// authorize 生成一次性 state 并返回微信授权地址
func (h *WechatLoginHandler) authorize(c *gin.Context, scene string, bindUserID int64) {
	cli := h.rt.WechatOAuth()
	if cli == nil || !cli.Available(scene) {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信登录未配置"})
		return
	}

	state := utils.GenerateRandomKey(16)
	payload, _ := json.Marshal(wechatState{Scene: scene, BindUserID: bindUserID})
	if err := redis.Set(wechatStatePrefix+state, string(payload), wechatStateTTL); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信登录初始化失败"})
		return
	}

	authorizeURL, err := cli.AuthorizeURL(scene, site.Platform().ConsoleBase()+wechatCallbackPath, state)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信登录未配置"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"authorize_url": authorizeURL}})
}

// bindWechat 按场景写入对应端的 openid（unionid/昵称有空值时保留已有值）
func (h *WechatLoginHandler) bindWechat(userID int64, u *upstream.WechatOAuthUser) error {
	if u.Scene == upstream.SceneMP {
		return h.userRepo.BindWechatMP(userID, u.OpenID, u.UnionID, u.Nickname)
	}
	return h.userRepo.BindWechatOpen(userID, u.OpenID, u.UnionID, u.Nickname)
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

// resolveWechatScene 解析登录场景：auto 按 UA 是否为微信内置浏览器判定。
// 页面内授权只能在微信内置浏览器打开，扫码只能在普通浏览器打开，两者互斥。
func resolveWechatScene(c *gin.Context, scene string) string {
	switch scene {
	case upstream.SceneMP, upstream.ScenePC:
		return scene
	default:
		if strings.Contains(c.GetHeader("User-Agent"), "MicroMessenger") {
			return upstream.SceneMP
		}
		return upstream.ScenePC
	}
}

// wechatLookupKeys 把授权结果映射为三列查询键：公众号 openid 与开放平台 openid 分属不同列
func wechatLookupKeys(u *upstream.WechatOAuthUser) (unionID, mpOpenID, openOpenID string) {
	if u.Scene == upstream.SceneMP {
		return u.UnionID, u.OpenID, ""
	}
	return u.UnionID, "", u.OpenID
}