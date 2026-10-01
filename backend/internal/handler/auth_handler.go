package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
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

// ---------- 两阶段下单：建单 / 查询 / 支付 / 取消 ----------

// orderDTO 订单对外结构。金额口径：amount 为外部支付金额、balance_amount 为余额抵扣额，
// total_amount = 两者之和即应付总额（建单未选渠道时 amount 即应付总额）。
func orderDTO(o *model.PaymentOrder) gin.H {
	data := gin.H{
		"pay_order_no":   o.PayOrderNo,
		"intent":         o.Intent,
		"status":         o.Status,
		"channel":        o.Channel,
		"amount":         o.Amount,
		"balance_amount": o.BalanceAmount,
		"total_amount":   o.Amount + o.BalanceAmount,
		"expire_time":    int64(0),
		"biz_no":         o.BizNo,
		"created_at":     o.CreatedAt,
	}
	if o.ExpireTime != nil {
		data["expire_time"] = o.ExpireTime.Unix()
	}
	if o.PaidAt != nil {
		data["paid_at"] = o.PaidAt
	}
	// 资源包订单：从业务单号拆出产品标识与资源包 ID，供前端展示与返回路径选择
	if o.Intent == service.PaymentIntentResourcePack {
		if product, packID, ok := splitPackBizNo(o.BizNo); ok {
			data["product"] = product
			data["pack_id"] = packID
		}
	}
	return data
}

// splitPackBizNo 解析资源包订单业务单号（`产品标识:资源包ID`）
func splitPackBizNo(bizNo string) (string, int64, bool) {
	product, idStr, ok := strings.Cut(bizNo, ":")
	if !ok {
		return "", 0, false
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return "", 0, false
	}
	return product, id, true
}

// orderPayMethods 订单可用支付方式：余额（仅购买资源包）+ 已启用的在线渠道
func (h *AuthHandler) orderPayMethods(intent string) []string {
	methods := make([]string, 0, 3)
	if intent == service.PaymentIntentResourcePack {
		methods = append(methods, model.ChannelBalance)
	}
	if h.alipay() != nil {
		methods = append(methods, model.ChannelAlipay)
	}
	if h.wechatPay() != nil {
		methods = append(methods, model.ChannelWechat)
	}
	return methods
}

// CreateOrder 建未支付订单（两阶段下单第一阶段：不扣余额、不选渠道、不调上游）
// POST /console/orders body: {intent, amount?, pack_id?, product?}
func (h *AuthHandler) CreateOrder(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Intent  string  `json:"intent" binding:"required"`
		Amount  float64 `json:"amount"`
		PackID  int64   `json:"pack_id"`
		Product string  `json:"product"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	order, err := h.balanceService.CreateOrder(userID, req.Intent, req.Product, req.PackID, req.Amount)
	if err != nil {
		h.respondPackPurchaseError(c, err)
		return
	}

	balance, _ := h.balanceService.GetUserBalance(userID)
	data := orderDTO(order)
	data["balance_available"] = balance
	data["pay_methods"] = h.orderPayMethods(order.Intent)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}

// ListOrders 我的待支付订单（「继续支付」入口）
// GET /console/orders
func (h *AuthHandler) ListOrders(c *gin.Context) {
	userID := c.GetInt64("user_id")

	orders, err := h.balanceService.ListUnpaidOrders(userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "查询订单失败"})
		return
	}
	balance, _ := h.balanceService.GetUserBalance(userID)

	list := make([]gin.H, 0, len(orders))
	for _, o := range orders {
		item := orderDTO(o)
		item["balance_available"] = balance
		item["pay_methods"] = h.orderPayMethods(o.Intent)
		list = append(list, item)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list}})
}

// GetOrder 订单详情（支付页展示与轮询共用；待支付单顺带按需向渠道对账一次）
// GET /console/orders/:pay_order_no
func (h *AuthHandler) GetOrder(c *gin.Context) {
	userID := c.GetInt64("user_id")

	order, err := h.balanceService.GetOrderForUser(userID, c.Param("pay_order_no"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 404, "message": "订单不存在"})
		return
	}
	if order.Status == 0 {
		if updated, rerr := h.balanceService.ReconcilePendingIfStale(order, h.alipay(), h.wechatPay()); rerr == nil && updated != nil {
			order = updated
		}
	}

	balance, _ := h.balanceService.GetUserBalance(userID)
	data := orderDTO(order)
	data["balance_available"] = balance
	data["pay_methods"] = h.orderPayMethods(order.Intent)
	if order.PayInfo != "" {
		var payInfo map[string]interface{}
		if err := json.Unmarshal([]byte(order.PayInfo), &payInfo); err == nil {
			data["pay_info"] = payInfo
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}

// PayOrder 选定支付方式并支付（两阶段下单第二阶段）
// POST /console/orders/:pay_order_no/pay body: {method, use_balance?, scene?}
// method=balance 走余额全额支付；method=alipay/wechat 时 use_balance（默认 true）决定是否先用余额抵扣；
// scene=h5 时微信走 H5 跳转，否则走 Native 扫码。
func (h *AuthHandler) PayOrder(c *gin.Context) {
	userID := c.GetInt64("user_id")
	payOrderNo := c.Param("pay_order_no")

	var req struct {
		Method     string `json:"method" binding:"required"`
		UseBalance *bool  `json:"use_balance"`
		Scene      string `json:"scene"` // 扫码类渠道场景：native-扫码（PC） h5-移动端跳转
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "参数错误"})
		return
	}

	if req.Method != model.ChannelBalance && req.Method != model.ChannelAlipay && req.Method != model.ChannelWechat {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "不支持的支付方式"})
		return
	}

	// 余额全额支付：直接结算
	if req.Method == model.ChannelBalance {
		h.respondBalancePaid(c, userID, payOrderNo)
		return
	}

	// 在线支付：默认勾选「使用余额抵扣」，取消勾选则全额走在线支付
	useBalance := true
	if req.UseBalance != nil {
		useBalance = *req.UseBalance
	}

	prep, err := h.balanceService.PrepareOrderOnlinePayment(userID, payOrderNo, req.Method, useBalance)
	if err != nil {
		h.respondPackPurchaseError(c, err)
		return
	}
	// 余额已覆盖全款：改走余额全额支付
	if prep.ExternalPart <= 0 {
		h.respondBalancePaid(c, userID, payOrderNo)
		return
	}

	order := prep.Order
	total := order.Amount + order.BalanceAmount
	data := gin.H{
		"pay_order_no": order.PayOrderNo,
		"amount":       order.Amount,
		"balance_part": order.BalanceAmount,
		"total_amount": total,
		"expire_time":  order.ExpireTime.Unix(),
		"channel":      order.Channel,
		"fully_paid":   false,
	}

	subject := "账户余额充值"
	if order.Intent == service.PaymentIntentResourcePack {
		subject = "购买资源包"
	}

	// 按渠道生成支付信息；上游失败回滚已抵扣余额并复位订单
	switch req.Method {
	case model.ChannelAlipay:
		if h.alipay() == nil {
			_ = h.balanceService.RevertOrderOnlinePayment(userID, order.PayOrderNo, total)
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "支付宝支付未配置"})
			return
		}
		payURL, err := h.alipay().BuildPagePayURL(order.PayOrderNo, order.Amount, subject, h.promotionService.SiteHosts(userID).ConsoleBase()+"/payment/")
		if err != nil {
			_ = h.balanceService.RevertOrderOnlinePayment(userID, order.PayOrderNo, total)
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "生成支付宝支付链接失败"})
			return
		}
		data["pay_type"] = "url"
		data["pay_url"] = payURL
	case model.ChannelWechat:
		if h.wechatPay() == nil {
			_ = h.balanceService.RevertOrderOnlinePayment(userID, order.PayOrderNo, total)
			c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付未配置"})
			return
		}
		if req.Scene == "h5" {
			h5URL, err := h.wechatPay().CreateH5Order(order.PayOrderNo, order.Amount, subject, c.ClientIP())
			if err != nil {
				log.Printf("微信 H5 下单失败 [pay_order_no=%s]: %v", order.PayOrderNo, err)
				_ = h.balanceService.RevertOrderOnlinePayment(userID, order.PayOrderNo, total)
				c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付下单失败"})
				return
			}
			data["pay_type"] = "h5"
			data["h5_url"] = h5URL
		} else {
			codeURL, err := h.wechatPay().CreateNativeOrder(order.PayOrderNo, order.Amount, subject)
			if err != nil {
				log.Printf("微信 Native 下单失败 [pay_order_no=%s]: %v", order.PayOrderNo, err)
				_ = h.balanceService.RevertOrderOnlinePayment(userID, order.PayOrderNo, total)
				c.JSON(http.StatusOK, gin.H{"code": 500, "message": "微信支付下单失败"})
				return
			}
			data["pay_type"] = "native"
			data["code_url"] = codeURL
		}
	}

	// 回填渠道支付信息（支付页刷新后按此恢复二维码/链接；失败则回滚已抵扣余额）
	payInfoBytes, err := json.Marshal(data)
	if err != nil || h.balanceService.BindOrderPayInfo(order.ID, string(payInfoBytes)) != nil {
		_ = h.balanceService.RevertOrderOnlinePayment(userID, order.PayOrderNo, total)
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "保存支付信息失败"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": data})
}

// respondBalancePaid 余额全额支付订单并返回发放的资源包
func (h *AuthHandler) respondBalancePaid(c *gin.Context, userID int64, payOrderNo string) {
	userPack, smsPack, err := h.balanceService.PayOrderWithBalance(userID, payOrderNo)
	if err != nil {
		h.respondPackPurchaseError(c, err)
		return
	}
	data := gin.H{"fully_paid": true}
	if userPack != nil {
		data["user_pack"] = userPack
	}
	if smsPack != nil {
		data["sms_user_pack"] = smsPack
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "支付成功", "data": data})
}

// CancelOrder 取消待支付订单（退还已抵扣的余额）
// POST /console/orders/:pay_order_no/cancel
func (h *AuthHandler) CancelOrder(c *gin.Context) {
	userID := c.GetInt64("user_id")
	if err := h.balanceService.CancelOrder(userID, c.Param("pay_order_no")); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "订单已取消"})
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
