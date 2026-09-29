package handler

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/runtime"
	"oemrpa/internal/service"
	"oemrpa/internal/utils"
)

var validator = utils.NewInputValidator()

// apiKeyID 取本次请求所用 API 密钥的 ID（仅 /v1/* 经 APIKeyMiddleware 的请求有值；控制台等登录态请求为 0）
func apiKeyID(c *gin.Context) int64 {
	if v, ok := c.Get("api"); ok {
		if cred, ok := v.(*model.ApiKey); ok && cred != nil {
			return cred.ID
		}
	}
	return 0
}

type UserHandler struct {
	userService      *service.UserService
	rt               *runtime.Runtime
	jwtManager       *utils.JWTManager
	authService      *service.AuthService
	loginLogRepo     *repository.LoginLogRepository
	promotionService *service.PromotionService
}

func NewUserHandler(
	userService *service.UserService,
	rt *runtime.Runtime,
	jwtManager *utils.JWTManager,
	authService *service.AuthService,
	loginLogRepo *repository.LoginLogRepository,
	promotionService *service.PromotionService,
) *UserHandler {
	return &UserHandler{
		userService:      userService,
		rt:               rt,
		jwtManager:       jwtManager,
		authService:      authService,
		loginLogRepo:     loginLogRepo,
		promotionService: promotionService,
	}
}

func (h *UserHandler) sms() *service.SMSService { return h.rt.SMS() }
func (h *UserHandler) captcha() service.CaptchaProvider {
	return h.rt.CaptchaProvider()
}

// captchaPayload 组装验证码校验参数：优先取通用字段 captcha_payload（极验/阿里云等通道），
// 为空时回落腾讯天御的票据与随机串旧字段，保证历史前端与插件调用不被破坏。
func captchaPayload(payload map[string]string, ticket, randstr string) map[string]string {
	if len(payload) > 0 {
		return payload
	}
	if ticket == "" && randstr == "" {
		return nil
	}
	return map[string]string{
		service.CaptchaFieldTicket:  ticket,
		service.CaptchaFieldRandStr: randstr,
	}
}

// SendCode 发送短信验证码
func (h *UserHandler) SendCode(c *gin.Context) {
	var req struct {
		Phone          string            `json:"phone" binding:"required"`
		CaptchaTicket  string            `json:"captcha_ticket"`  // 腾讯天御：票据（兼容旧字段）
		CaptchaRand    string            `json:"captcha_randstr"` // 腾讯天御：随机串（兼容旧字段）
		CaptchaPayload map[string]string `json:"captcha_payload"` // 各通道通用验证参数（极验/阿里云等）
		Scene          string            `json:"scene" binding:"required"` // register / login / change_password
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 验证人机验证码
	remoteIP := c.ClientIP()
	err := h.captcha().Verify(captchaPayload(req.CaptchaPayload, req.CaptchaTicket, req.CaptchaRand), remoteIP)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "captcha verification failed",
		})
		return
	}

	// 发送短信验证码
	err = h.sms().SendVerificationCode(req.Phone)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 验证码应存入 Redis，设置 5 分钟过期（当前版本使用第三方短信服务商的验证功能）

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"expire_in": 300,
		},
	})
}

// Register 用户注册
func (h *UserHandler) Register(c *gin.Context) {
	var req struct {
		Phone          string `json:"phone" binding:"required"`
		Username       string `json:"username" binding:"required"`
		SMSCode        string `json:"sms_code" binding:"required"`
		Password       string `json:"password" binding:"required"`
		CaptchaTicket  string            `json:"captcha_ticket"`  // 腾讯天御：票据（兼容旧字段）
		CaptchaRandstr string            `json:"captcha_randstr"` // 腾讯天御：随机串（兼容旧字段）
		CaptchaPayload map[string]string `json:"captcha_payload"` // 各通道通用验证参数
		Ref            string            `json:"ref"`             // 推广码（推广链接 ?ref=XXXX，12 位数字+小写字母）；命中时按码绑定归属，未命中=平台直营
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 校验用户名格式（仅支持英文+数字+下划线）
	if !service.ValidateUsername(req.Username) {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "用户名仅支持英文、数字、下划线，长度3-32位",
		})
		return
	}

	// 验证人机验证码
	remoteIP := c.ClientIP()
	err := h.captcha().Verify(captchaPayload(req.CaptchaPayload, req.CaptchaTicket, req.CaptchaRandstr), remoteIP)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "captcha verification failed",
		})
		return
	}

	// 验证短信验证码
	valid, err := h.sms().VerifyCode(req.Phone, req.SMSCode)
	if err != nil || !valid {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "短信验证码错误或已过期",
		})
		return
	}

	// 注册归属：仅认推广码（推广链接 ?ref=XXXX）——先查用户型推广码，再查员工推广码（须启用）。
	// 不信任客户端直接指定的推广方 ID，避免归属被伪造；未命中时按平台直营注册。
	referrerType, referrerID, _ := h.promotionService.ResolveAffCode(req.Ref)

	// 注册用户
	user, err := h.userService.Register(req.Phone, req.Username, req.Password, referrerType, referrerID)
	if err != nil {
		switch err {
		case service.ErrPhoneAlreadyExists:
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "该手机号已被注册",
			})
			return
		case service.ErrUsernameAlreadyExists:
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "该用户名已被占用",
			})
			return
		case service.ErrInvalidUsername:
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "用户名仅支持英文、数字、下划线，长度3-32位",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "registration failed",
		})
		return
	}

	// 生成 Token
	token, err := h.jwtManager.GenerateToken(user.ID, user.Phone, "user", 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to generate token",
		})
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

// Login 用户登录（支持用户名/手机号）
func (h *UserHandler) Login(c *gin.Context) {
	var req struct {
		Account        string `json:"account" binding:"required"`
		Password       string `json:"password"`
		SMSCode        string `json:"sms_code"`
		LoginType      string            `json:"login_type" binding:"required"` // password / sms_code
		CaptchaTicket  string            `json:"captcha_ticket"`                // 腾讯天御：票据（兼容旧字段）
		CaptchaRandstr string            `json:"captcha_randstr"`               // 腾讯天御：随机串（兼容旧字段）
		CaptchaPayload map[string]string `json:"captcha_payload"`               // 各通道通用验证参数
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 检测SQL注入
	if err := validator.DetectSQLInjection(req.Account); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid input detected",
		})
		return
	}

	// 验证登录类型
	if req.LoginType != "password" && req.LoginType != "sms_code" {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid login type",
		})
		return
	}

	// 验证码登录必须使用手机号（短信发送依赖手机号）
	if req.LoginType == "sms_code" {
		if err := validator.ValidatePhone(req.Account); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "验证码登录请输入正确的手机号",
			})
			return
		}
	}

	// 验证人机验证码
	remoteIP := c.ClientIP()
	err := h.captcha().Verify(captchaPayload(req.CaptchaPayload, req.CaptchaTicket, req.CaptchaRandstr), remoteIP)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "captcha verification failed",
		})
		return
	}

	// 根据登录方式验证（login_type 已在上方校验，仅 password/sms_code 两种）
	var user *model.User
	var loginErr error
	switch req.LoginType {
	case "password":
		user, loginErr = h.userService.Login(req.Account, req.Password)
	case "sms_code":
		// 验证短信验证码
		valid, verr := h.sms().VerifyCode(req.Account, req.SMSCode)
		if verr != nil || !valid {
			loginErr = errors.New("短信验证码错误或已过期")
		} else {
			user, loginErr = h.userService.GetUserByPhone(req.Account)
			if loginErr != nil {
				loginErr = service.ErrInvalidPassword
			} else if user.Status == 0 {
				loginErr = service.ErrUserDisabled
			}
		}
	default:
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid login type",
		})
		return
	}

	// 记录登录日志（成功与失败均记录）
	if loginErr != nil {
		h.loginLogRepo.InsertUserLoginLog(&model.UserLoginLog{
			Account:    req.Account,
			LoginType:  req.LoginType,
			IP:         c.ClientIP(),
			UserAgent:  c.GetHeader("User-Agent"),
			Status:     0,
			FailReason: loginErr.Error(),
		})
		c.JSON(http.StatusOK, gin.H{
			"code":    401,
			"message": loginErr.Error(),
		})
		return
	}

	h.loginLogRepo.InsertUserLoginLog(&model.UserLoginLog{
		UserID:    user.ID,
		Account:   req.Account,
		LoginType: req.LoginType,
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Status:    1,
	})

	// 生成 Token
	token, err := h.jwtManager.GenerateToken(user.ID, user.Phone, "user", 24*time.Hour)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to generate token",
		})
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

// GetProfile 获取用户信息
func (h *UserHandler) GetProfile(c *gin.Context) {
	userID := c.GetInt64("user_id")

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "user not found",
		})
		return
	}

	// 脱敏处理
	kycName := ""
	kycNumber := ""
	if user.RealnameStatus != model.RealnameNone {
		if user.VerifiedName.Valid {
			kycName = maskName(user.VerifiedName.String)
		}
		if user.VerifiedNumber.Valid {
			kycNumber = maskIDCard(user.VerifiedNumber.String)
		}
	}

	// 账户实名剩余免费认证次数（终身累计，按已发起核验次数计）
	freeAuthRemaining := 0
	if h.authService != nil {
		if remaining, err := h.authService.GetFreeAuthRemaining(userID); err == nil {
			freeAuthRemaining = remaining
		}
	}
	// 企业实名剩余免费认证次数（各自独立，按已发起核验次数计）
	enterpriseFreeAuthRemaining := 0
	if h.authService != nil {
		if remaining, err := h.authService.GetKybFreeAuthRemaining(userID); err == nil {
			enterpriseFreeAuthRemaining = remaining
		}
	}

	// 当前生效的 API 密钥（密钥对存于 api 表；完整 api_secret 仅返回给用户本人展示/复制）
	apiKey, apiSecret, apiPermission := "", "", ""
	if ak, err := h.userService.GetAPIKeyByUser(userID); err == nil {
		apiKey = ak.APIKey
		apiPermission = ak.Permission
		if user.RealnameStatus != model.RealnameNone {
			apiSecret = ak.APISecret
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"user_id":         user.ID,
			"phone":           user.Phone,
			"username":        user.Username,
			"realname_status": user.RealnameStatus,
			"verified_name":   kycName,
			"verified_number": kycNumber,
			"balance":         user.Balance,
			"api_key":         apiKey,
			"api_secret":      apiSecret,
			"api_permission":  apiPermission,
			"created_at":      user.CreatedAt,
			// 账户实名免费认证次数（free_auth_limit 终身免费上限，free_auth_remaining 剩余免费次数）
			"free_auth_limit":     3,
			"free_auth_remaining": freeAuthRemaining,
			// 企业实名免费认证次数（各自独立 3 次，超次按 KYC_ENTERPRISE_PRICE 计费）
			"enterprise_free_auth_limit":     3,
			"enterprise_free_auth_remaining": enterpriseFreeAuthRemaining,
			"kyc_personal_price":             h.authService.GetKycPersonalPriceForDisplay(),
			"kyc_enterprise_price":           h.authService.GetKycEnterprisePriceForDisplay(),
		},
	})
}

// buildAPIPermission 由「全部/部分 + 端点清单」构造 api.permission 值。
// scopeType 为 all 时返回 "all"；为 partial 时返回去重后的端点标识（逗号分隔），
// 端点须为已登记的端点标识且至少选择一项，否则返回 false。
func buildAPIPermission(scopeType string, endpoints []string) (string, bool) {
	if scopeType == "all" {
		return "all", true
	}
	seen := make(map[string]bool)
	list := make([]string, 0, len(endpoints))
	for _, e := range endpoints {
		e = strings.TrimSpace(e)
		if e == "" {
			continue
		}
		if model.APIEndpointByCode(e) == nil {
			return "", false
		}
		if seen[e] {
			continue
		}
		seen[e] = true
		list = append(list, e)
	}
	if len(list) == 0 {
		return "", false
	}
	return strings.Join(list, ","), true
}

// ListAPIKeys 查询当前用户全部 API 密钥及可授权的端点目录
func (h *UserHandler) ListAPIKeys(c *gin.Context) {
	userID := c.GetInt64("user_id")

	list, err := h.userService.ListAPIKeys(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "查询 API 密钥失败"})
		return
	}

	items := make([]gin.H, 0, len(list))
	for _, k := range list {
		items = append(items, gin.H{
			"id":         k.ID,
			"name":       k.Name,
			"api_key":    k.APIKey,
			"api_secret": k.APISecret,
			"permission": k.Permission,
			"created_at": k.CreatedAt,
		})
	}

	endpoints := make([]gin.H, 0, len(model.APIEndpoints))
	for _, ep := range model.APIEndpoints {
		endpoints = append(endpoints, gin.H{"code": ep.Code, "title": ep.Title, "product": ep.Product})
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      items,
			"endpoints": endpoints,
			"max":       service.MaxAPIKeysPerUser,
		},
	})
}

// CreateAPIKey 创建 API 密钥（需先完成实名认证，可自定义名称与权限范围）
func (h *UserHandler) CreateAPIKey(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Name      string   `json:"name"`
		ScopeType string   `json:"scope_type"` // all-全部端点 partial-部分端点
		Endpoints []string `json:"endpoints"`  // scope_type=partial 时的端点标识清单
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}

	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "user not found"})
		return
	}
	if user.RealnameStatus == model.RealnameNone {
		c.JSON(http.StatusOK, gin.H{"code": 403, "message": "请先完成实名认证后创建 API 密钥"})
		return
	}

	permission, ok := buildAPIPermission(req.ScopeType, req.Endpoints)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请至少选择一个可调用的 API"})
		return
	}

	k, err := h.userService.CreateAPIKey(userID, strings.TrimSpace(req.Name), permission)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"id":         k.ID,
			"name":       k.Name,
			"api_key":    k.APIKey,
			"api_secret": k.APISecret,
			"permission": k.Permission,
			"created_at": k.CreatedAt,
		},
	})
}

// UpdateAPIKey 修改指定 API 密钥的名称与权限范围
func (h *UserHandler) UpdateAPIKey(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid api key id"})
		return
	}

	var req struct {
		Name      string   `json:"name"`
		ScopeType string   `json:"scope_type"`
		Endpoints []string `json:"endpoints"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}

	permission, ok := buildAPIPermission(req.ScopeType, req.Endpoints)
	if !ok {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请至少选择一个可调用的 API"})
		return
	}

	if err := h.userService.UpdateAPIKey(userID, id, strings.TrimSpace(req.Name), permission); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "修改 API 密钥失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"permission": permission}})
}

// DeleteAPIKey 删除指定 API 密钥
func (h *UserHandler) DeleteAPIKey(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid api key id"})
		return
	}

	if err := h.userService.DeleteAPIKey(userID, id); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "删除 API 密钥失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// ChangePassword 修改密码
func (h *UserHandler) ChangePassword(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		SMSCode        string            `json:"sms_code" binding:"required"`
		NewPassword    string            `json:"new_password" binding:"required"`
		CaptchaTicket  string            `json:"captcha_ticket"`  // 腾讯天御：票据（兼容旧字段）
		CaptchaRandstr string            `json:"captcha_randstr"` // 腾讯天御：随机串（兼容旧字段）
		CaptchaPayload map[string]string `json:"captcha_payload"` // 各通道通用验证参数
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 验证新密码强度
	if err := validator.ValidatePassword(req.NewPassword); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "password must be 8-32 characters with letters and numbers",
		})
		return
	}

	// 验证人机验证码
	remoteIP := c.ClientIP()
	err := h.captcha().Verify(captchaPayload(req.CaptchaPayload, req.CaptchaTicket, req.CaptchaRandstr), remoteIP)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "captcha verification failed",
		})
		return
	}

	// 验证短信验证码
	user, err := h.userService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "user not found",
		})
		return
	}
	valid, err := h.sms().VerifyCode(user.Phone, req.SMSCode)
	if err != nil || !valid {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "短信验证码错误或已过期",
		})
		return
	}

	err = h.userService.ChangePassword(userID, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to change password",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
	})
}

// 辅助函数
func maskName(name string) string {
	if len(name) == 0 {
		return ""
	}
	if len(name) == 1 {
		return name
	}
	if len(name) == 2 {
		return name[0:1] + "*"
	}
	return name[0:1] + "**" + name[len(name)-1:]
}

func maskIDCard(idCard string) string {
	if len(idCard) < 8 {
		return idCard
	}
	return idCard[0:3] + "***********" + idCard[len(idCard)-4:]
}
