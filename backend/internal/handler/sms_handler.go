package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/model"
	"oemrpa/internal/service"
	"oemrpa/internal/upstream"
)

// SMSHandler 短信业务对外接口
type SMSHandler struct {
	sms *service.SmsChannelService
}

func NewSMSHandler(sms *service.SmsChannelService) *SMSHandler {
	return &SMSHandler{sms: sms}
}

// SubmitSign 用户提交短信签名（Web 登录态，含资质材料与联麓报备字段）
// POST /console/sms/sign
func (h *SMSHandler) SubmitSign(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		SignName       string `json:"sign_name" binding:"required"` // 签名内容
		Label          int    `json:"label"`                        // 资质类型：1-营业执照 2-商标
		CreditCodeURL  string `json:"credit_code_url"`              // 营业执照图片 URL
		IDCardFront    string `json:"id_card_front"`                // 身份证正面 URL
		IDCardBack     string `json:"id_card_back"`                 // 身份证反面 URL
		Company        string `json:"company"`                      // 公司名称（他公司必填）
		LegalPerson    string `json:"legal_person"`                 // 法人姓名（他公司必填）
		CreditCode     string `json:"credit_code"`                  // 统一社会信用代码（他公司必填）
		CreditUserName string `json:"credit_user_name"`             // 经办人姓名（他公司必填）
		IDCard         string `json:"id_card"`                      // 经办人身份证号（他公司必填）
		Phone          string `json:"phone"`                        // 经办人手机号（他公司必填）
		SxCommits      string `json:"sx_commits"`                   // 用户接收意愿承诺函 URL
		AuthLetter     string `json:"auth_letter"`                  // 短信签名授权书 URL
		Screenshot     string `json:"screenshot"`                   // 商标备案截图 URL（label=2 必填）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名内容不能为空"})
		return
	}

	// 清理 URL 字段两端可能被前端带上的反引号/空白，避免上游将其判为非法 URL 而报材料为空
	trimURL := func(s string) string { return strings.TrimSpace(strings.Trim(s, "`")) }
	req.CreditCodeURL = trimURL(req.CreditCodeURL)
	req.IDCardFront = trimURL(req.IDCardFront)
	req.IDCardBack = trimURL(req.IDCardBack)
	req.SxCommits = trimURL(req.SxCommits)
	req.AuthLetter = trimURL(req.AuthLetter)
	req.Screenshot = trimURL(req.Screenshot)

	resp, err := h.sms.CreateSign(userID, &model.SmsSign{
		SignName:       req.SignName,
		SignType:       2, // 平台代下游客户报备，签名来源固定为他公司
		Label:          req.Label,
		CreditCodeURL:  req.CreditCodeURL,
		IDCardFront:    req.IDCardFront,
		IDCardBack:     req.IDCardBack,
		Company:        req.Company,
		LegalPerson:    req.LegalPerson,
		CreditCode:     req.CreditCode,
		CreditUserName: req.CreditUserName,
		IDCard:         req.IDCard,
		Phone:          req.Phone,
		SxCommits:      req.SxCommits,
		AuthLetter:     req.AuthLetter,
		Screenshot:     req.Screenshot,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	signID := ""
	if resp != nil {
		signID = strconv.FormatInt(resp.SignId, 10)
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"sign_id": signID,
		},
	})
}

// GetSignDetail 查询单条签名详情（用户侧，供修改页回填表单）
// GET /console/sms/signs/:id
func (h *SMSHandler) GetSignDetail(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名 ID 无效"})
		return
	}

	sign, err := h.sms.GetSign(userID, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if sign == nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": sign})
}

// UpdateSign 修改短信签名（仅已通过/驳回的签名可改；联麓无修改接口，通过创建新签名+删除旧签名实现）
// POST /console/sms/signs/:id/update
func (h *SMSHandler) UpdateSign(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名 ID 无效"})
		return
	}

	var req struct {
		SignName       string `json:"sign_name" binding:"required"`
		Label          int    `json:"label"`
		CreditCodeURL  string `json:"credit_code_url" binding:"required"`
		IDCardFront    string `json:"id_card_front" binding:"required"`
		IDCardBack     string `json:"id_card_back" binding:"required"`
		Company        string `json:"company" binding:"required"`
		LegalPerson    string `json:"legal_person" binding:"required"`
		CreditCode     string `json:"credit_code" binding:"required"`
		CreditUserName string `json:"credit_user_name" binding:"required"`
		IDCard         string `json:"id_card" binding:"required"`
		Phone          string `json:"phone" binding:"required"`
		SxCommits      string `json:"sx_commits"`
		AuthLetter     string `json:"auth_letter" binding:"required"`
		Screenshot     string `json:"screenshot"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名内容不能为空"})
		return
	}

	trimURL := func(s string) string { return strings.TrimSpace(strings.Trim(s, "`")) }
	updated, err := h.sms.UpdateSign(userID, id, &model.SmsSign{
		SignName:       req.SignName,
		SignType:       2, // 联麓统一他公司
		Label:          req.Label,
		CreditCodeURL:  trimURL(req.CreditCodeURL),
		IDCardFront:    trimURL(req.IDCardFront),
		IDCardBack:     trimURL(req.IDCardBack),
		Company:        req.Company,
		LegalPerson:    req.LegalPerson,
		CreditCode:     req.CreditCode,
		CreditUserName: req.CreditUserName,
		IDCard:         req.IDCard,
		Phone:          req.Phone,
		SxCommits:      trimURL(req.SxCommits),
		AuthLetter:     trimURL(req.AuthLetter),
		Screenshot:     trimURL(req.Screenshot),
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"sign_id": updated.UpSignID,
			"status":  updated.Status,
		},
	})
}

// ListSigns 用户签名申请列表（分页）
// GET /console/sms/signs?page=1&page_size=10
func (h *SMSHandler) ListSigns(c *gin.Context) {
	userID := c.GetInt64("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	list, total, err := h.sms.ListSigns(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list, "total": total}})
}

// CreateTemplate 用户提交模板（本地存表待审核）
// POST /console/sms/templates
func (h *SMSHandler) CreateTemplate(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		TemplateName    string `json:"template_name" binding:"required"`    // 模板名称
		TemplateContent string `json:"template_content" binding:"required"` // 模板内容（含变量）
		// verify-验证码 notify-通知 marketing-营销
		TemplateType    string `json:"template_type" binding:"required"`
		SignID          int64  `json:"sign_id"`                             // 绑定签名 id（上游报备模板必填，须为已审核通过的签名）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板名称、内容、类型不能为空"})
		return
	}
	if req.TemplateType != "verify" && req.TemplateType != "notify" && req.TemplateType != "marketing" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "暂不支持该模板类型"})
		return
	}

	if err := h.sms.CreateTemplate(userID, &model.SmsTemplate{
		TemplateName:    req.TemplateName,
		TemplateContent: req.TemplateContent,
		TemplateType:    req.TemplateType,
		SignID:          req.SignID,
	}); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// UpdateTemplate 修改模板（控制台用户）：重新报备上游并重置为待审核
// PUT /console/sms/templates/:id
func (h *SMSHandler) UpdateTemplate(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板ID非法"})
		return
	}

	var req struct {
		TemplateName    string `json:"template_name" binding:"required"`    // 模板名称
		TemplateContent string `json:"template_content" binding:"required"` // 模板内容（含变量）
		TemplateType    string `json:"template_type" binding:"required"`    // verify-验证码 notify-通知 marketing-营销
		SignID          int64  `json:"sign_id"`                             // 绑定签名 id（须为已审核通过的签名）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板名称、内容、类型不能为空"})
		return
	}
	if req.TemplateType != "verify" && req.TemplateType != "notify" && req.TemplateType != "marketing" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "暂不支持该模板类型"})
		return
	}

	t, err := h.sms.UpdateTemplate(userID, id, &model.SmsTemplate{
		TemplateName:    req.TemplateName,
		TemplateContent: req.TemplateContent,
		TemplateType:    req.TemplateType,
		SignID:          req.SignID,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": t})
}

// ListTemplates 用户模板列表（分页）
// GET /console/sms/templates?page=1&page_size=10
func (h *SMSHandler) ListTemplates(c *gin.Context) {
	userID := c.GetInt64("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	list, total, err := h.sms.ListTemplates(userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list, "total": total}})
}

// GetTemplateDetail 查询单条模板详情（用户侧，供修改页回填表单）
// GET /console/sms/templates/:id
func (h *SMSHandler) GetTemplateDetail(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板 ID 无效"})
		return
	}

	t, err := h.sms.GetTemplate(userID, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	if t == nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板不存在"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": t})
}

// FillTemplateID 用户回填模板 ID（上游控制台创建后填写）
// POST /console/sms/templates/:id/template-id
func (h *SMSHandler) FillTemplateID(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板 ID 无效"})
		return
	}
	var req struct {
		TemplateID string `json:"template_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板 ID 不能为空"})
		return
	}

	if err := h.sms.FillTemplateID(userID, id, req.TemplateID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// QuerySignStatus 手动查询签名审核状态（用户侧）：调上游核对审核结果并回写本地
// POST /console/sms/signs/:id/query
func (h *SMSHandler) QuerySignStatus(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名ID非法"})
		return
	}

	sign, err := h.sms.QuerySignStatus(userID, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": sign})
}

// QueryTemplateStatus 手动查询模板审核状态（用户侧）：调上游核对审核结果并回写本地
// POST /console/sms/templates/:id/query
func (h *SMSHandler) QueryTemplateStatus(c *gin.Context) {
	userID := c.GetInt64("user_id")
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板ID非法"})
		return
	}

	t, err := h.sms.QueryTemplateStatus(userID, id)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": t})
}

// ListSendRecords 用户发送记录（分页 + 日期筛选）
// GET /console/sms/records?page=1&page_size=10&start_date=2026-09-01&end_date=2026-09-11
func (h *SMSHandler) ListSendRecords(c *gin.Context) {
	userID := c.GetInt64("user_id")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "10"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	list, total, err := h.sms.ListSendRecords(userID,
		c.DefaultQuery("start_date", ""), c.DefaultQuery("end_date", ""), page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": list, "total": total}})
}

// SMSStats 短信统计（总条数/总金额 + 按日趋势）
// GET /console/sms/stats?days=7
func (h *SMSHandler) SMSStats(c *gin.Context) {
	userID := c.GetInt64("user_id")
	days, _ := strconv.Atoi(c.DefaultQuery("days", "7"))
	if days < 1 || days > 90 {
		days = 7
	}

	stats, err := h.sms.SMSStats(userID, days)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": stats})
}

// SendSMSForAPI 下游透传发送短信（API Key 鉴权，permission 为 all 或 sms_send）
// POST /v1/sms/send
func (h *SMSHandler) SendSMSForAPI(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		PhoneNumberSet []string `json:"phone_number_set"` // 目标号码
		TemplateID     string   `json:"template_id"`      // 模板ID
		TemplateParams []string `json:"template_params"`  // 模板参数
		SignID         int64    `json:"sign_id"`          // 已审核签名 ID（优先，按主键直取，省掉按名检索）
		SignName       string   `json:"sign_name"`        // 已审核签名内容（兼容旧调用；sign_id 优先）
		SmsType        string   `json:"sms_type"`
		SessionContext string   `json:"session_context"`
		NotifyURL      string   `json:"notify_url"` // 回执主动推送地址（可选）
	}
	_ = c.ShouldBindJSON(&req) // 转发型接口，入参交给上游校验

	if req.NotifyURL != "" && !strings.HasPrefix(req.NotifyURL, "http://") && !strings.HasPrefix(req.NotifyURL, "https://") {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "notify_url 必须以 http:// 或 https:// 开头"})
		return
	}

	// 集合类字段序列化为 JSON 字符串随 body 上送（参与签名黑名单，不上送签名）
	phoneSetJSON, _ := json.Marshal(req.PhoneNumberSet)
	paramSetJSON, _ := json.Marshal(req.TemplateParams)

	resp, err := h.sms.SendSMS(userID, apiKeyID(c), req.SignID, &upstream.SendSmsRequest{
		PhoneNumberSet:   string(phoneSetJSON),
		TemplateId:       req.TemplateID,
		SignName:         req.SignName,
		TemplateParamSet: string(paramSetJSON),
		SmsType:          req.SmsType,
		SessionContext:   req.SessionContext,
	}, req.NotifyURL)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"request_id":  resp.MessageSid,
			"message_sid": resp.TaskId,
		},
	})
}

// SendSMSForWeb 控制台在线发送短信（登录用户，按自己账号的签名/模板校验并计费）
// POST /console/sms/send
func (h *SMSHandler) SendSMSForWeb(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		PhoneNumberSet []string `json:"phone_number_set"` // 目标号码
		TemplateID     string   `json:"template_id"`      // 已审核通过的模板（本地主键或上游模板 ID）
		TemplateParams []string `json:"template_params"`  // 模板参数（按模板变量顺序）
		SignName       string   `json:"sign_name"`        // 已审核通过的签名（留空时用模板绑定的签名）
	}
	if err := c.ShouldBindJSON(&req); err != nil || len(req.PhoneNumberSet) == 0 || req.TemplateID == "" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "手机号与短信模板不能为空"})
		return
	}
	for _, phone := range req.PhoneNumberSet {
		if err := authValidator.ValidatePhone(strings.TrimSpace(phone)); err != nil {
			c.JSON(http.StatusOK, gin.H{"code": 400, "message": "手机号格式不正确：" + phone})
			return
		}
	}

	// 集合类字段序列化为 JSON 字符串上送（与下游 API 一致）
	phoneSetJSON, _ := json.Marshal(req.PhoneNumberSet)
	paramSetJSON, _ := json.Marshal(req.TemplateParams)

	resp, err := h.sms.SendSMS(userID, 0, 0, &upstream.SendSmsRequest{
		PhoneNumberSet:   string(phoneSetJSON),
		TemplateId:       req.TemplateID,
		SignName:         req.SignName,
		TemplateParamSet: string(paramSetJSON),
	}, "")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"request_id":  resp.MessageSid,
			"message_sid": resp.TaskId,
		},
	})
}

// templateStatusForAPI 平台模板状态 → 下游（魔方）模板状态：0待审→1审核中 1通过→2通过 2驳回→3未通过
func templateStatusForAPI(status int) int {
	switch status {
	case 1:
		return 2
	case 2:
		return 3
	default:
		return 1
	}
}

// templateIDForAPI 返回模板标识：优先上游模板 ID（发送时按该 ID 解析渲染），无则用本地主键
func templateIDForAPI(t *model.SmsTemplate) string {
	if t.TemplateID != "" {
		return t.TemplateID
	}
	return strconv.FormatInt(t.ID, 10)
}

// CreateSignForAPI 下游 API 创建短信签名（API Key 鉴权）：资质材料以 URL 形式传入，
// 平台不做图片处理/存储，原样透传上游报备。
// POST /v1/sms/signs
func (h *SMSHandler) CreateSignForAPI(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		SignName       string `json:"sign_name" binding:"required"`        // 签名内容
		Label          int    `json:"label"`                               // 资质类型：1-营业执照 2-商标
		CreditCodeURL  string `json:"credit_code_url" binding:"required"`  // 营业执照图片 URL
		IDCardFront    string `json:"id_card_front" binding:"required"`    // 身份证正面图片 URL
		IDCardBack     string `json:"id_card_back" binding:"required"`     // 身份证反面图片 URL
		Company        string `json:"company" binding:"required"`          // 公司名称（他公司主体）
		LegalPerson    string `json:"legal_person" binding:"required"`     // 法人姓名
		CreditCode     string `json:"credit_code" binding:"required"`      // 统一社会信用代码
		CreditUserName string `json:"credit_user_name" binding:"required"` // 经办人姓名
		IDCard         string `json:"id_card" binding:"required"`          // 经办人身份证号
		Phone          string `json:"phone" binding:"required"`            // 经办人手机号
		SxCommits      string `json:"sx_commits"`                          // 意愿承诺函 URL（选填，未传时上游以营业执照兜底）
		AuthLetter     string `json:"auth_letter" binding:"required"`      // 短信签名授权书 URL
		Screenshot     string `json:"screenshot"`                          // 商标备案截图 URL（label=2 时必填）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "签名内容、企业主体与资质图片 URL 不能为空"})
		return
	}

	// 图片 URL 原样透传上游，平台不下载/转存；仅清理两端可能混入的反引号与空白
	trimURL := func(s string) string { return strings.TrimSpace(strings.Trim(s, "`")) }
	sign := &model.SmsSign{
		SignName:       req.SignName,
		SignType:       2, // 平台代下游客户报备，签名来源固定为他公司
		Label:          req.Label,
		CreditCodeURL:  trimURL(req.CreditCodeURL),
		IDCardFront:    trimURL(req.IDCardFront),
		IDCardBack:     trimURL(req.IDCardBack),
		Company:        req.Company,
		LegalPerson:    req.LegalPerson,
		CreditCode:     req.CreditCode,
		CreditUserName: req.CreditUserName,
		IDCard:         req.IDCard,
		Phone:          req.Phone,
		SxCommits:      trimURL(req.SxCommits),
		AuthLetter:     trimURL(req.AuthLetter),
		Screenshot:     trimURL(req.Screenshot),
	}

	if _, err := h.sms.CreateSign(userID, sign); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"sign_id":    sign.ID,       // 平台签名记录 ID
			"up_sign_id": sign.UpSignID, // 上游签名 ID（报备模板时绑定用）
			"status":     sign.Status,   // 0-待审核 1-审核中 2-通过 3-驳回
		},
	})
}

// CreateTemplateForAPI 下游 API 创建短信模板（API Key 鉴权，平台模板型）
// POST /v1/sms/templates
func (h *SMSHandler) CreateTemplateForAPI(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Title    string `json:"title" binding:"required"`   // 模板标题
		Content  string `json:"content" binding:"required"` // 模板内容（@ 变量占位，由魔方端转换 @var(name)）
		SmsType  string `json:"sms_type"`                   // verify-验证码 notify-通知 marketing-营销
		SignID   int64  `json:"sign_id"`                    // 关联签名 ID（优先，按主键直取；可为账号自有签名或公共签名）
		SignName string `json:"sign_name"`                  // 关联签名内容（兼容旧调用；sign_id 优先）
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板标题与内容不能为空"})
		return
	}
	if req.SmsType == "" {
		req.SmsType = "notify"
	}

	t, err := h.sms.CreateTemplateForAPI(userID, req.Title, req.Content, req.SmsType, req.SignID, req.SignName)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"template": gin.H{
				"template_id":     templateIDForAPI(t),
				"template_status": templateStatusForAPI(t.Status),
			},
		},
	})
}

// resolveDownstreamTemplate 解析下游模板记录：id 支持本地主键，也支持创建接口返回的上游模板 ID
// （创建接口返回的 template_id 优先取上游模板 ID）。未命中时已写出 400 响应，返回 nil。
func (h *SMSHandler) resolveDownstreamTemplate(c *gin.Context, userID int64) *model.SmsTemplate {
	param := c.Param("id")
	id, _ := strconv.ParseInt(param, 10, 64)
	t, err := h.sms.GetTemplateForAPI(userID, id, param)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return nil
	}
	if t == nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板不存在"})
		return nil
	}
	return t
}

// GetTemplateForAPI 下游 API 查询模板状态（支持本地主键或上游模板 ID）
// GET /v1/sms/templates/:id
func (h *SMSHandler) GetTemplateForAPI(c *gin.Context) {
	userID := c.GetInt64("user_id")
	t := h.resolveDownstreamTemplate(c, userID)
	if t == nil {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"template": gin.H{
				"template_id":     templateIDForAPI(t),
				"template_status": templateStatusForAPI(t.Status),
				"msg":             t.Reason,
			},
		},
	})
}

// UpdateTemplateForAPI 下游 API 修改模板内容（重置待审核并重新报备上游）
// PUT /v1/sms/templates/:id
func (h *SMSHandler) UpdateTemplateForAPI(c *gin.Context) {
	userID := c.GetInt64("user_id")
	tpl := h.resolveDownstreamTemplate(c, userID)
	if tpl == nil {
		return
	}
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "模板内容不能为空"})
		return
	}

	t, err := h.sms.UpdateTemplateForAPI(userID, tpl.ID, req.Content)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"template": gin.H{
				"template_id":     templateIDForAPI(t),
				"template_status": templateStatusForAPI(t.Status),
			},
		},
	})
}

// DeleteTemplateForAPI 下游 API 删除模板
// DELETE /v1/sms/templates/:id
func (h *SMSHandler) DeleteTemplateForAPI(c *gin.Context) {
	userID := c.GetInt64("user_id")
	tpl := h.resolveDownstreamTemplate(c, userID)
	if tpl == nil {
		return
	}
	if err := h.sms.DeleteTemplateForAPI(userID, tpl.ID); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// QuerySmsReport 下游 API 查询短信发送回执（按上游 taskId）
// POST /v1/sms/report
func (h *SMSHandler) QuerySmsReport(c *gin.Context) {
	userID := c.GetInt64("user_id")
	var req struct {
		TaskId   string `json:"task_id" binding:"required"` // 上游任务ID（发送接口返回的 request_id / message_sid）
		PageNo   int    `json:"page_no"`
		PageSize int    `json:"page_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "task_id 不能为空"})
		return
	}

	items, err := h.sms.PullReportForUser(userID, req.TaskId, req.PageNo, req.PageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"task_id": req.TaskId,
			"reports": items,
		},
	})
}

// QueryReplies 查询短信上行回复（下游主动查询）。
// 提供 date（yyyyMMdd）时平台先按日向上游拉取回复并落库，再返回本用户回复记录。
// POST /v1/sms/replies
func (h *SMSHandler) QueryReplies(c *gin.Context) {
	userID := c.GetInt64("user_id")

	var req struct {
		Date     string `json:"date"` // 回复日期 yyyyMMdd（可选，提供时先向上游拉取同步）
		TaskID   string `json:"task_id"`
		PageNo   int    `json:"page_no"`
		PageSize int    `json:"page_size"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "请求参数错误"})
		return
	}
	page, pageSize := req.PageNo, req.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}

	list, total, err := h.sms.QueryReplies(userID, req.Date, req.TaskID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}

	replies := make([]gin.H, 0, len(list))
	for _, r := range list {
		replies = append(replies, gin.H{
			"task_id":      r.TaskID,
			"phone":        r.Phone,
			"sequence_id":  r.SequenceID,
			"content_down": r.ContentDown,
			"content_up":   r.ContentUp,
			"resp_time":    r.RespTime,
			"status":       r.Status,
			"tag":          r.Tag,
		})
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"total":   total,
			"replies": replies,
		},
	})
}
