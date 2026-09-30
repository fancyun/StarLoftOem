package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/runtime"
	"oemrpa/internal/service"
	"oemrpa/internal/site"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"
)

var authValidator = utils.NewInputValidator()

type AuthHandler struct {
	authService         *service.AuthService
	balanceService      *service.BalanceService
	promotionService    *service.PromotionService
	resourcePackRepo    *repository.ResourcePackRepository
	smsResourcePackRepo *repository.SmsResourcePackRepository
	rt                  *runtime.Runtime
}

func NewAuthHandler(
	authService *service.AuthService,
	balanceService *service.BalanceService,
	promotionService *service.PromotionService,
	resourcePackRepo *repository.ResourcePackRepository,
	smsResourcePackRepo *repository.SmsResourcePackRepository,
	rt *runtime.Runtime,
) *AuthHandler {
	return &AuthHandler{
		authService:         authService,
		balanceService:      balanceService,
		promotionService:    promotionService,
		resourcePackRepo:    resourcePackRepo,
		smsResourcePackRepo: smsResourcePackRepo,
		rt:                  rt,
	}
}

func (h *AuthHandler) alipay() *upstream.AlipayClient { return h.rt.Alipay() }

func (h *AuthHandler) wechatPay() *upstream.WechatPayClient { return h.rt.WechatPay() }

// StartFvAuthForAPI 发起 FV 人脸识别认证（下游 API 调用）
// product: 具体子产品（fv_auth / fv_self），返回本平台自站链接（PC 扫码 / 移动端跳转）
func (h *AuthHandler) StartFvAuthForAPI(product string) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetInt64("user_id")

		var req struct {
			Name         string `json:"name"`
			IDCard       string `json:"id_card"`
			ReturnURL    string `json:"return_url"`
			NotifyURL    string `json:"notify_url"` // 结果主动推送地址（可选：不传则不推送，下游用 /v1/fv/result 轮询校对）
			BizExtraData string `json:"biz_extra_data"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
			return
		}

		// 有源人脸核验必须提供姓名与证件号；无源人脸仅需活体识别
		if product == model.ServiceFVAuth {
			if err := authValidator.ValidateName(req.Name); err != nil {
				c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid name format"})
				return
			}
			if err := authValidator.ValidateIDCardMinAge(req.IDCard, utils.MinKycAge); err != nil {
				c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
				return
			}
		}

		// return_url 可选：为空表示核身后不回跳下游，停留平台承接页展示认证结果
		if req.ReturnURL != "" {
			if err := authValidator.ValidateURL(req.ReturnURL); err != nil {
				c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid return_url format"})
				return
			}
		}
		// notify_url 可选：为空表示结果不主动推送，由下游轮询 /v1/fv/result 主动校对
		// （下游插件「跳过平台异步通知」模式即为此用法，不得因此拒绝请求）
		if req.NotifyURL != "" {
			if err := authValidator.ValidateURL(req.NotifyURL); err != nil {
				c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid notify_url format"})
				return
			}
		}
		req.BizExtraData = authValidator.SanitizeString(req.BizExtraData)

		result, err := h.authService.StartFvAuth(userID, apiKeyID(c), product, req.Name, req.IDCard, req.ReturnURL, req.NotifyURL, req.BizExtraData)
		if err != nil {
			if err == service.ErrInsufficientBalance {
				c.JSON(http.StatusOK, gin.H{"code": 400, "message": "insufficient balance"})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"biz_no":       result.Record.BizNo,
				"site_url":     result.AuthURL,
				"expired_in":   authExpiredIn(result.Record),
				"expired_time": authExpiredAt(result.Record),
			},
		})
	}
}

// GetFvBestImg 获取认证记录的活体最佳图（下游 API 调用）
// POST /v1/fv/best-img
func (h *AuthHandler) GetFvBestImg(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		BizNo string `json:"biz_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "biz_no 不能为空"})
		return
	}

	img, err := h.authService.GetRecordBestImg(userID, req.BizNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"biz_no":   req.BizNo,
			"best_img": img,
		},
	})
}

// GetFvMedia 获取认证记录已自动保存的照片/视频（base64，保存 30 天）
// POST /v1/fv/media
func (h *AuthHandler) GetFvMedia(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		BizNo string `json:"biz_no" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "biz_no 不能为空"})
		return
	}

	media, err := h.authService.GetRecordMedia(userID, req.BizNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"biz_no": req.BizNo,
			"media":  media,
		},
	})
}

// QueryFvResult 按业务流水号查询认证记录状态（下游 API 轮询/结果校对）
// POST /v1/fv/result
func (h *AuthHandler) QueryFvResult(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req struct {
		BizNo string `json:"biz_no" binding:"required"`
		Sync  int    `json:"sync"` // 1=立即校对一次（下游手动查询按钮；仍受每单查询次数上限约束）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "biz_no 不能为空"})
		return
	}

	record, err := h.authService.GetRecordStatus(userID, req.BizNo, req.Sync == 1)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"biz_no":         record.BizNo,
			"status":         record.Status,
			"result_code":    record.ResultCode,
			"result_message": record.ResultMessage,
		},
	})
}

// StartFvAuthForWeb 发起人脸核验认证（Console Web 用户）
// POST /console/fv
func (h *AuthHandler) StartFvAuthForWeb(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Product   string `json:"product" binding:"required"` // fv_auth / fv_self
		Name      string `json:"name"`
		IDCard    string `json:"id_card"`
		ReturnURL string `json:"return_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请指定人脸核验产品"})
		return
	}
	if req.Product != model.ServiceFVAuth && req.Product != model.ServiceFVSelf {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "无效的人脸核验产品，仅支持 fv_auth / fv_self"})
		return
	}
	if req.Product == model.ServiceFVAuth {
		if err := authValidator.ValidateName(req.Name); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "有源人脸核验需提供合法姓名"})
			return
		}
		if err := authValidator.ValidateIDCardMinAge(req.IDCard, utils.MinKycAge); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
			return
		}
	}
	if req.ReturnURL == "" {
		req.ReturnURL = h.promotionService.SiteHosts(userID).ConsoleBase() + "/user/fv"
	}

	result, err := h.authService.StartFvAuth(userID, apiKeyID(c), req.Product, req.Name, req.IDCard, req.ReturnURL, "", "")
	if err != nil {
		if err == service.ErrInsufficientBalance {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "余额不足"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"biz_no":       result.Record.BizNo,
			"site_url":     result.AuthURL,
			"expired_in":   authExpiredIn(result.Record),
			"expired_time": authExpiredAt(result.Record),
		},
	})
}

// authExpiredAt 返回核验链接到期时间戳（Unix 秒）：优先取上游返回的 token 到期时间，
// 上游未返回时回退「记录创建时间 + 15 分钟」。
func authExpiredAt(record *model.AuthRecord) int64 {
	if record != nil && record.TokenExpireAt != nil && !record.TokenExpireAt.IsZero() {
		return record.TokenExpireAt.Unix()
	}
	if record == nil {
		return time.Now().Add(15 * time.Minute).Unix()
	}
	return record.CreatedAt.Add(15 * time.Minute).Unix()
}

// authExpiredIn 返回核验链接剩余有效秒数（已过期返回 0）
func authExpiredIn(record *model.AuthRecord) int {
	remain := int(time.Until(time.Unix(authExpiredAt(record), 0)).Seconds())
	if remain < 0 {
		return 0
	}
	return remain
}

// HandleFvReturn 处理 FV 自站 return 回调（公开端点，无需 API Key）
// 接收回跳带回的本平台业务号（biz_no，由发起时拼进 return_url），主动向上游校对一次，再 302 到下游 return_url
func (h *AuthHandler) HandleFvReturn(c *gin.Context) {
	bizNo := c.Query("biz_no")

	redirect, err := h.authService.HandleFvReturn(bizNo)
	if err != nil {
		log.Printf("FV return 处理失败: +%v", err)
		c.Redirect(http.StatusFound, site.Platform().ServiceBase())
		return
	}
	c.Redirect(http.StatusFound, redirect)
}

// GetFvPageStatus 查询 FV 承接页记录状态（公开端点，无需登录，凭上游核身 token 查询）
// GET /service/fv/status?token=xxx
// 供承接页轮询：记录进入终态后展示认证结果，不再继续展示二维码
func (h *AuthHandler) GetFvPageStatus(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "缺少核身令牌（token）"})
		return
	}

	// sync=1：用户点击「我已完成核身」时的强制校对（跳过最小间隔，仍受每单查询次数上限约束）
	record, err := h.authService.GetFvRecordByToken(token, c.Query("sync") == "1")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "认证记录不存在"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"biz_no":         record.BizNo,
			"status":         record.Status,
			"finished":       record.Status >= 2, // 2已完成 3失败 4已取消 5超时结束 6发起失败 均为终态
			"result_code":    record.ResultCode,
			"result_message": record.ResultMessage,
			"return_url":     record.ReturnURL,
		},
	})
}

// GetUserAuthStatus 查询用户的 KYC 认证状态（Web 前端调用）
func (h *AuthHandler) GetUserAuthStatus(c *gin.Context) {
	userID := c.GetInt64("user_id")

	// 获取用户信息
	user, err := h.authService.GetUserByID(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "user not found",
		})
		return
	}

	// 获取最新 kyc 记录
	kycRecord, _ := h.authService.GetLatestKycRecord(userID)

	// 提取 user 实名信息
	kycName := ""
	if user.VerifiedName.Valid {
		kycName = user.VerifiedName.String
	}
	kycNumber := ""
	if user.VerifiedNumber.Valid {
		kycNumber = user.VerifiedNumber.String
	}

	// 构建响应：包含 user 实名状态 + kyc 最新记录
	resp := gin.H{
		"realname_status": user.RealnameStatus,
		"verified_name":   kycName,
		"verified_number": kycNumber,
	}

	if kycRecord != nil {
		resp["record_status"] = kycRecord.Status
		resp["record_name"] = kycRecord.Name
		resp["record_id_card"] = kycRecord.IDCard
		resp["record_id"] = kycRecord.ID
	} else {
		resp["record_status"] = -1 // 无记录
	}

	// 状态为进行中时返回实名单信息（账户实名走腾讯云人脸核身，核身会话一次有效，不提供继续认证地址）
	if kycRecord != nil && kycRecord.Status == 1 {
		if kycRecord.BizNo != "" {
			resp["pending_biz_no"] = kycRecord.BizNo
		}
		// 返回下游的 return_url（认证完成后跳转回下游地址）
		if kycRecord.ReturnURL != "" {
			resp["return_url"] = kycRecord.ReturnURL
		}
	}

	// 如果已有结果（状态 2/3），也返回 return_url 用于结果页跳转
	if kycRecord != nil && (kycRecord.Status == 2 || kycRecord.Status == 3) {
		if resp["return_url"] == nil && kycRecord.ReturnURL != "" {
			resp["return_url"] = kycRecord.ReturnURL
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    resp,
	})
}

// SyncKycResult 同步上游认证结果
func (h *AuthHandler) SyncKycResult(c *gin.Context) {
	userID := c.GetInt64("user_id")

	record, err := h.authService.SyncKycRecord(userID)
	if err != nil {
		// 没有进行中的认证记录，返回当前状态
		kycRecord, _ := h.authService.GetLatestKycRecord(userID)
		if kycRecord != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    0,
				"message": "success",
				"data": gin.H{
					"synced":        false,
					"record_status": kycRecord.Status,
				},
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "success",
			"data": gin.H{
				"synced":        false,
				"record_status": -1,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"synced":        true,
			"record_status": record.Status,
			"up_token":      record.UpToken,
			"return_url":    record.ReturnURL,
		},
	})
}

// StartKybAuthForWeb 发起企业实名（Web 前端调用）
func (h *AuthHandler) StartKybAuthForWeb(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		CompanyName string `json:"company_name" binding:"required"`
		CreditCode  string `json:"credit_code" binding:"required"`
		LegalName   string `json:"legal_name" binding:"required"`
		LegalIDCard string `json:"legal_id_card" binding:"required"`
		ReturnURL   string `json:"return_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请填写完整的企业与法人信息"})
		return
	}

	result, err := h.authService.StartKybAuth(
		userID, req.CompanyName, req.CreditCode, req.LegalName, req.LegalIDCard, req.ReturnURL,
	)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"auth_url": result.AuthURL,
		},
	})
}

// GetKybStatus 查询企业实名认证状态（Web 前端调用）
func (h *AuthHandler) GetKybStatus(c *gin.Context) {
	userID := c.GetInt64("user_id")

	// 前端轮询时先尝试同步腾讯云人脸核身结果（法人扫脸）
	_ = h.authService.SyncKybResult(userID)

	// 企业实名剩余免费认证次数（供前端展示）
	freeRemaining := 0
	if remaining, err := h.authService.GetKybFreeAuthRemaining(userID); err == nil {
		freeRemaining = remaining
	}
	// 未配置工商四要素核验能力时走人工审核，前端据此切换表单与文案
	manualReview := !h.authService.KybSelfServiceAvailable()

	rec, err := h.authService.GetKybRecord(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code": 0,
			"data": gin.H{
				"record_status":       -1, // 无企业实名记录
				"free_auth_remaining": freeRemaining,
				"manual_review":       manualReview,
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"company_name":        rec.CompanyName,
			"credit_code":         rec.CreditCode,
			"four_factor":         rec.FourFactorStatus,
			"record_status":       rec.Status,
			"pending_auth_url":    h.authService.BuildKybAuthURL(rec),
			"free_auth_remaining": freeRemaining,
			"manual_review":       manualReview,
			"result_message":      rec.ResultMessage,
		},
	})
}

// SubmitKybManual 提交企业实名人工审核申请（未配置工商四要素核验能力时使用）
func (h *AuthHandler) SubmitKybManual(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		CompanyName string `json:"company_name" binding:"required"`
		CreditCode  string `json:"credit_code" binding:"required"`
		LegalName   string `json:"legal_name" binding:"required"`
		LegalIDCard string `json:"legal_id_card" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请填写完整的企业与法人信息"})
		return
	}

	if err := h.authService.SubmitKybManualReview(
		userID, req.CompanyName, req.CreditCode, req.LegalName, req.LegalIDCard,
	); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已提交，请等待人工审核"})
}

// CancelKycRecord 取消当前进行中的认证记录
func (h *AuthHandler) CancelKycRecord(c *gin.Context) {
	userID := c.GetInt64("user_id")

	err := h.authService.CancelKycRecord(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "认证已取消",
	})
}

// StartAuthForWeb Web 端发起 KYC 认证（账户实名）
func (h *AuthHandler) StartAuthForWeb(c *gin.Context) {
	userID := c.GetInt64("user_id")

	// 从 JSON 读取参数
	var req struct {
		Name      string `json:"name" binding:"required"`
		IDCard    string `json:"id_card" binding:"required"`
		ReturnURL string `json:"return_url"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "name and id_card are required",
		})
		return
	}

	// 设置默认回调地址（Web 账户实名：认证完成后跳回账户实名页）
	if req.ReturnURL == "" {
		req.ReturnURL = "/certification"
	}
	// notifyURL 留空，由 service 层使用写死的上游异步通知地址（finAuthNotifyURL）
	notifyURL := ""

	// 年龄限制：平台实名需年满16周岁
	if err := authValidator.ValidateIDCardMinAge(req.IDCard, utils.MinKycAge); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	// 标记这是用户实名认证
	bizExtraData := fmt.Sprintf(`{"type":"user_auth","user_id":%d}`, userID)

	// 发起认证（账户实名 source=1：不写认证记录、不计费，实名信息单独储存在系统库实名记录表）
	result, err := h.authService.StartAuth(
		userID,
		"", // 账户实名，product 参数不再使用
		req.Name,
		req.IDCard,
		req.ReturnURL,
		notifyURL,
		bizExtraData,
		1,    // 账户实名
		true, // 账户实名（source=1）固定为免费
	)
	if err != nil {
		if err == service.ErrInsufficientBalance {
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "insufficient balance",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"biz_no":       result.KycRecord.BizNo,
			"auth_url":     result.AuthURL,
			"expired_time": result.KycRecord.CreatedAt.Add(15 * time.Minute).Unix(),
			"expired_in":   900,
		},
	})
}

// GetUserAuthRecords 查询用户的认证记录列表（带分页）
func (h *AuthHandler) GetUserAuthRecords(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Page      int    `json:"page" form:"page"`
		PageSize  int    `json:"page_size" form:"page_size"`
		StartDate string `json:"start_date" form:"start_date"` // 开始日期 YYYY-MM-DD（可选，按创建时间筛选）
		EndDate   string `json:"end_date" form:"end_date"`     // 结束日期 YYYY-MM-DD（可选，按创建时间筛选）
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 设置默认值
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}
	if req.PageSize > 100 {
		req.PageSize = 100
	}

	records, total, err := h.authService.GetUserAuthRecords(userID, req.Page, req.PageSize, req.StartDate, req.EndDate)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to query records",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":      records,
			"total":     total,
			"page":      req.Page,
			"page_size": req.PageSize,
		},
	})
}

// GetUserAuthCallStats 统计用户近30天的认证调用次数
func (h *AuthHandler) GetUserAuthCallStats(c *gin.Context) {
	userID := c.GetInt64("user_id")

	dates, counts, err := h.authService.GetUserAuthCallStats(userID, 30)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to query call stats",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"dates":  dates,
			"counts": counts,
		},
	})
}

// CreateRecharge 发起充值（在线支付渠道：支付宝电脑网站支付 / 微信扫码与 H5）
func (h *AuthHandler) CreateRecharge(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Amount  float64 `json:"amount" binding:"required,gt=0"`
		Channel string  `json:"channel" binding:"required"`
		Scene   string  `json:"scene"` // 扫码类渠道场景：native-扫码（PC） h5-移动端跳转
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	if req.Channel != model.ChannelAlipay && req.Channel != model.ChannelWechat {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "不支持的支付渠道",
		})
		return
	}

	// 构造待支付单（不落库）：先调上游生成支付信息，成功后再落库
	order, err := h.balanceService.PrepareRecharge(userID, req.Amount, req.Channel)
	if err != nil {
		if errors.Is(err, service.ErrDailyOnlineLimitExceeded) {
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": err.Error(),
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to create recharge order",
		})
		return
	}

	// 已存在未过期的待支付单（支付查重命中）：直接复用首次下单时保存的渠道支付信息，不重复调上游/建单
	if order.ID > 0 {
		if order.PayInfo == "" {
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "支付信息缺失，请重新发起"})
			return
		}
		var payInfo map[string]interface{}
		if err := json.Unmarshal([]byte(order.PayInfo), &payInfo); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "支付信息解析失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": payInfo})
		return
	}

	data := gin.H{
		"pay_order_no": order.PayOrderNo,
		"amount":       order.Amount,
		"expire_time":  order.ExpireTime.Unix(),
		"channel":      order.Channel,
	}

	// 按渠道生成支付信息（上游失败不落库，不产生支付记录）
	switch req.Channel {
	case model.ChannelAlipay:
		if h.alipay() == nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    500,
				"message": "支付宝支付未配置",
			})
			return
		}
		payURL, err := h.alipay().BuildPagePayURL(order.PayOrderNo, order.Amount, "账户余额充值", h.promotionService.SiteHosts(userID).ConsoleBase()+"/payment/")
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    500,
				"message": "生成支付宝支付链接失败",
			})
			return
		}
		data["pay_type"] = "url"
		data["pay_url"] = payURL
	case model.ChannelWechat:
		if h.wechatPay() == nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    500,
				"message": "微信支付未配置",
			})
			return
		}
		if req.Scene == "h5" {
			h5URL, err := h.wechatPay().CreateH5Order(order.PayOrderNo, order.Amount, "账户余额充值", c.ClientIP())
			if err != nil {
				log.Printf("微信 H5 下单失败 [pay_order_no=%s]: %v", order.PayOrderNo, err)
				c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付下单失败"})
				return
			}
			data["pay_type"] = "h5"
			data["h5_url"] = h5URL
		} else {
			codeURL, err := h.wechatPay().CreateNativeOrder(order.PayOrderNo, order.Amount, "账户余额充值")
			if err != nil {
				log.Printf("微信 Native 下单失败 [pay_order_no=%s]: %v", order.PayOrderNo, err)
				c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付下单失败"})
				return
			}
			data["pay_type"] = "native"
			data["code_url"] = codeURL
		}
	}

	// 保存渠道支付信息到支付单（供支付查重命中时直接复用返回）
	payInfoBytes, err := json.Marshal(data)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存支付信息失败"})
		return
	}
	order.PayInfo = string(payInfoBytes)

	// 上游成功：落库支付单
	if err := h.balanceService.CreateRechargeOrder(order); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "保存充值订单失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// GetRechargeResult 查询充值结果（轮询查询支付订单状态）
func (h *AuthHandler) GetRechargeResult(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		PayOrderNo string `json:"pay_order_no" form:"pay_order_no" binding:"required"`
	}

	if err := c.ShouldBindQuery(&req); err != nil {
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "invalid request parameters",
			})
			return
		}
	}

	// 查询支付订单
	order, err := h.balanceService.GetPaymentOrder(req.PayOrderNo)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    404,
			"message": "order not found",
		})
		return
	}

	// 检查订单是否属于该用户
	if order.UserID != userID {
		c.JSON(http.StatusOK, gin.H{
			"code":    403,
			"message": "permission denied: order does not belong to you",
		})
		return
	}

	// 待支付时主动向渠道确认真实状态并落地（幂等，30 秒内每单最多一次），
	// 避免异步回调延迟/丢失导致用户支付成功后界面仍显示未支付
	if order.Status == 0 {
		if fresh, rerr := h.balanceService.ReconcilePendingIfStale(order, h.alipay(), h.wechatPay()); rerr == nil && fresh != nil {
			order = fresh
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"pay_order_no":     order.PayOrderNo,
			"amount":           order.Amount,
			"status":           order.Status,
			"channel":          order.Channel,
			"channel_trade_no": order.ChannelTradeNo,
			"paid_at":          order.PaidAt,
		},
	})
}

// ---------- 资源包（Web 用户） ----------

// GetRefundableOrders 查询用户可提现（可退款）的已支付充值订单（Console 提现页）
func (h *AuthHandler) GetRefundableOrders(c *gin.Context) {
	userID := c.GetInt64("user_id")
	orders, err := h.balanceService.GetRefundableOrders(userID)
	if err != nil {
		log.Printf("查询可提现订单失败: user_id=%d, err=%v", userID, err)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "查询可提现订单失败"})
		return
	}
	total := 0.0
	list := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		refundable := o.Amount - o.RefundAmount
		total += refundable
		list = append(list, gin.H{
			"id":            o.ID,
			"pay_order_no":  o.PayOrderNo,
			"amount":        o.Amount,
			"refund_amount": o.RefundAmount,
			"refundable":    refundable,
			"channel":       o.Channel,
			"created_at":    o.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total": total,
			"list":  list,
		},
	})
}

// Withdraw 提现：将余额按充值支付订单原路退回（支付宝/微信退款，可部分退款、自动拆分多个订单）
// POST /console/withdraw
func (h *AuthHandler) Withdraw(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Amount float64 `json:"amount" binding:"required,gt=0"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请输入正确的提现金额"})
		return
	}
	if req.Amount > 1000000 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "单次提现金额不能超过 1000000 元"})
		return
	}

	results, err := h.balanceService.Withdraw(userID, req.Amount, h.alipay(), h.wechatPay())
	if err != nil {
		if err == service.ErrInsufficientBalance {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "余额不足，无法提现"})
			return
		}
		log.Printf("提现失败: user_id=%d, amount=%.2f, err=%v", userID, req.Amount, err)
		if results != nil {
			// 部分订单已成功退款：返回明细，提示剩余未完成
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error(), "data": gin.H{"list": results, "partial": true}})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	log.Printf("提现成功: user_id=%d, amount=%.2f, orders=%d", userID, req.Amount, len(results))
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "提现成功，款项将原路退回",
		"data": gin.H{
			"list": results,
		},
	})
}

// ListResourcePacks 在售资源包列表（可选按 product 筛选，如 ?product=sms）
// prices 为该用户适用的售价（推广/单用户定价优先），前端按此展示与下单。
func (h *AuthHandler) ListResourcePacks(c *gin.Context) {
	userID := c.GetInt64("user_id")
	status := 1
	packs, err := h.resourcePackRepo.ListPacks(&status)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get resource packs",
		})
		return
	}
	product := c.Query("product")
	if product != "" {
		filtered := packs[:0]
		for _, p := range packs {
			if p.Product == product {
				filtered = append(filtered, p)
			}
		}
		packs = filtered
	}
	prices := make(map[int64]float64, len(packs))
	for _, p := range packs {
		prices[p.ID] = h.promotionService.PackPrice(userID, p.Product, p.ID, p.Price)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list":   packs,
			"prices": prices,
		},
	})
}

// PurchaseResourcePack 使用余额购买资源包（不支持直接为资源包付费，需先充值再购买）
func (h *AuthHandler) PurchaseResourcePack(c *gin.Context) {
	userID := c.GetInt64("user_id")
	packID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || packID <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid pack id",
		})
		return
	}

	up, err := h.promotionService.PurchasePack(userID, packID)
	if err != nil {
		switch err {
		case service.ErrInsufficientBalance:
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "余额不足，请先充值",
			})
		case repository.ErrPackOffSale:
			c.JSON(http.StatusOK, gin.H{
				"code":    400,
				"message": "资源包已下架",
			})
		case repository.ErrPackNotFound:
			c.JSON(http.StatusOK, gin.H{
				"code":    404,
				"message": "资源包不存在",
			})
		default:
			c.JSON(http.StatusOK, gin.H{
				"code":    500,
				"message": "购买资源包失败",
			})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "购买成功",
		"data": gin.H{
			"user_pack": up,
		},
	})
}

// respondPackPurchaseError 资源包购买失败响应：下架/不存在/推广 余额不足/单日限额等业务原因原样提示
func (h *AuthHandler) respondPackPurchaseError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, repository.ErrPackOffSale):
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "资源包已下架"})
	case errors.Is(err, repository.ErrPackNotFound):
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "资源包不存在"})
	case errors.Is(err, service.ErrInsufficientBalance),
		errors.Is(err, service.ErrDailyOnlineLimitExceeded):
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
	default:
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "购买资源包失败"})
	}
}

// PurchaseResourcePackOnline 在线购买资源包（支持组合支付：余额支付一部分 + 支付宝支付剩余部分）
func (h *AuthHandler) PurchaseResourcePackOnline(c *gin.Context) {
	userID := c.GetInt64("user_id")
	packID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || packID <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid pack id",
		})
		return
	}

	var req struct {
		Channel string `json:"channel" binding:"required"`
		Scene   string `json:"scene"` // 扫码类渠道场景：native-扫码（PC） h5-移动端跳转
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    400,
			"message": "invalid request parameters",
		})
		return
	}

	// 归属推广商的用户：资源包购买走平台余额（与平台直营一致），成交后由推广服务为其推广商计提提成
	if h.promotionService.BillingReferrerID(userID) > 0 {
		up, err := h.promotionService.PurchasePack(userID, packID)
		if err != nil {
			h.respondPackPurchaseError(c, err)
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "购买成功",
			"data": gin.H{
				"user_pack":      up,
				"fully_paid":     true,
				"balance_amount": 0,
			},
		})
		return
	}

	result, err := h.balanceService.PrepareResourcePackOnline(userID, packID, req.Channel)
	if err != nil {
		h.respondPackPurchaseError(c, err)
		return
	}

	// 余额已全额支付，直接发放资源包
	if result.FullyPaidByBalance {
		c.JSON(http.StatusOK, gin.H{
			"code":    0,
			"message": "购买成功",
			"data": gin.H{
				"user_pack":      result.UserPack,
				"fully_paid":     true,
				"balance_amount": result.BalanceAmount,
			},
		})
		return
	}

	order := result.PaymentOrder

	// 已存在未过期的待支付单（支付查重命中）：直接复用首次下单时保存的渠道支付信息，不重复调上游/扣余额/建单
	if order.ID > 0 {
		if order.PayInfo == "" {
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "支付信息缺失，请重新发起"})
			return
		}
		var payInfo map[string]interface{}
		if err := json.Unmarshal([]byte(order.PayInfo), &payInfo); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "支付信息解析失败"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": payInfo})
		return
	}

	data := gin.H{
		"pay_order_no": order.PayOrderNo,
		"amount":       result.ExternalAmount,
		"balance_part": result.BalanceAmount,
		"expire_time":  order.ExpireTime.Unix(),
		"channel":      order.Channel,
		"fully_paid":   false,
	}

	// 按渠道生成支付信息（上游失败回滚已扣余额，不落库支付单）
	switch req.Channel {
	case model.ChannelAlipay:
		if h.alipay() == nil {
			_ = h.balanceService.RollbackResourcePackReservation(result)
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "支付宝支付未配置"})
			return
		}
		payURL, err := h.alipay().BuildPagePayURL(order.PayOrderNo, order.Amount, "购买资源包", h.promotionService.SiteHosts(userID).ConsoleBase()+"/payment/")
		if err != nil {
			_ = h.balanceService.RollbackResourcePackReservation(result)
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成支付宝支付链接失败"})
			return
		}
		data["pay_type"] = "url"
		data["pay_url"] = payURL
	case model.ChannelWechat:
		if h.wechatPay() == nil {
			_ = h.balanceService.RollbackResourcePackReservation(result)
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付未配置"})
			return
		}
		if req.Scene == "h5" {
			h5URL, err := h.wechatPay().CreateH5Order(order.PayOrderNo, order.Amount, "购买资源包", c.ClientIP())
			if err != nil {
				log.Printf("微信 H5 下单失败 [pay_order_no=%s]: %v", order.PayOrderNo, err)
				_ = h.balanceService.RollbackResourcePackReservation(result)
				c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付下单失败"})
				return
			}
			data["pay_type"] = "h5"
			data["h5_url"] = h5URL
		} else {
			codeURL, err := h.wechatPay().CreateNativeOrder(order.PayOrderNo, order.Amount, "购买资源包")
			if err != nil {
				log.Printf("微信 Native 下单失败 [pay_order_no=%s]: %v", order.PayOrderNo, err)
				_ = h.balanceService.RollbackResourcePackReservation(result)
				c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付下单失败"})
				return
			}
			data["pay_type"] = "native"
			data["code_url"] = codeURL
		}
	}

	// 保存渠道支付信息到支付单（供支付查重命中时直接复用返回）
	payInfoBytes, err := json.Marshal(data)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存支付信息失败"})
		return
	}
	order.PayInfo = string(payInfoBytes)

	// 上游成功：落库支付单 + 余额部分账单（失败回滚已扣余额）
	if err := h.balanceService.ConfirmResourcePackOrder(result); err != nil {
		_ = h.balanceService.RollbackResourcePackReservation(result)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存支付单失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// MyResourcePacks 我的资源包列表
func (h *AuthHandler) MyResourcePacks(c *gin.Context) {
	userID := c.GetInt64("user_id")
	packs, err := h.resourcePackRepo.ListUserPacks(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"code":    500,
			"message": "failed to get my resource packs",
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"list": packs,
		},
	})
}

// ListSmsResourcePacks 在售短信资源包列表（短信库独立表，含验证码/通知与营销两类）
// prices 为该用户适用的售价（推广/单用户定价优先，按资源包类型分别取覆盖价）
func (h *AuthHandler) ListSmsResourcePacks(c *gin.Context) {
	userID := c.GetInt64("user_id")
	status := 1
	packs, err := h.smsResourcePackRepo.ListPacks(&status, "")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "failed to get sms resource packs"})
		return
	}
	prices := make(map[int64]float64, len(packs))
	for _, p := range packs {
		prices[p.ID] = h.promotionService.PackPrice(userID, p.Product, p.ID, p.Price)
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success", "data": gin.H{"list": packs, "prices": prices},
	})
}

// PurchaseSmsResourcePack 使用余额购买短信资源包
func (h *AuthHandler) PurchaseSmsResourcePack(c *gin.Context) {
	userID := c.GetInt64("user_id")
	packID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || packID <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid pack id"})
		return
	}

	up, err := h.promotionService.PurchaseSmsPack(userID, packID)
	if err != nil {
		switch err {
		case service.ErrInsufficientBalance:
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "余额不足，请先充值"})
		case repository.ErrPackOffSale:
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "资源包已下架"})
		case repository.ErrPackNotFound:
			c.JSON(http.StatusOK, gin.H{"code": 404, "message": "资源包不存在"})
		default:
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "购买资源包失败"})
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "购买成功", "data": gin.H{"user_pack": up},
	})
}

// MySmsResourcePacks 我的短信资源包列表
func (h *AuthHandler) MySmsResourcePacks(c *gin.Context) {
	userID := c.GetInt64("user_id")
	packs, err := h.smsResourcePackRepo.ListUserPacks(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "failed to get my sms resource packs"})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "success", "data": gin.H{"list": packs},
	})
}
