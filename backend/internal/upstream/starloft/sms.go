package starloft

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"oemrpa/internal/upstream"
)

// 上游状态码空间（本包对外统一口径，供业务层按同一套映射回写本地状态）：
//
//	签名：1-审核通过 2-待审核 3-驳回 4-报备中
//	模板：1-审核通过 2-待审核 3-驳回
const (
	upstreamStatusApproved = 1
	upstreamStatusPending  = 2
	upstreamStatusRejected = 3
)

// MarketingAvailable 营销短信通道是否可用：由后台配置项决定（StarLoft 侧须已开通营销通道）
func (c *Client) MarketingAvailable() bool {
	return c != nil && c.marketingEnabled
}

// SendSms 发送短信：POST /v1/sms/send。
// StarLoft 平台仅支持模板发送；回执与回复由平台推送到本平台回调地址（notify_url）。
// 返回的 message_sid 为平台任务号，与本平台发送记录一一对应，用于回执/回复落库时关联。
func (c *Client) SendSms(req *upstream.SendSmsRequest) (*upstream.SendSmsResponse, error) {
	if req.TemplateId == "" {
		return nil, fmt.Errorf("仅支持模板发送，请先创建并审核通过短信模板")
	}
	if req.Content != "" {
		return nil, fmt.Errorf("仅支持模板发送，不支持直接发送短信内容")
	}
	var phones, params []string
	if err := json.Unmarshal([]byte(req.PhoneNumberSet), &phones); err != nil || len(phones) == 0 {
		return nil, fmt.Errorf("手机号码不能为空")
	}
	_ = json.Unmarshal([]byte(req.TemplateParamSet), &params)
	if params == nil {
		params = []string{}
	}

	payload := map[string]interface{}{
		"phone_number_set": phones,
		"template_id":      req.TemplateId,
		"template_params":  params,
		"sms_type":         req.SmsType,
		"session_context":  req.SessionContext,
		"notify_url":       c.smsNotifyURL(),
	}
	if req.SignName != "" {
		payload["sign_name"] = req.SignName
	}

	var out struct {
		RequestID  string `json:"request_id"`
		MessageSid string `json:"message_sid"`
	}
	if err := c.call(http.MethodPost, "/v1/sms/send", "sms-send", req.SessionContext, payload, &out); err != nil {
		return nil, err
	}

	sid := out.MessageSid
	if sid == "" {
		sid = out.RequestID
	}
	if sid == "" {
		return nil, fmt.Errorf("StarLoft 平台未返回短信任务号")
	}
	// 平台不返回预扣费条数：Count 置 0，由业务层沿用本地估算条数计费
	return &upstream.SendSmsResponse{TaskId: sid, MessageSid: sid, Status: "00", Message: "success"}, nil
}

// CreateSign 创建短信签名：POST /v1/sms/signs（资质材料以 URL 透传，平台不处理图片）。
// 返回的 SignId 为平台签名记录主键，业务层以其作为「上游签名 ID」用于后续绑定模板与状态查询。
func (c *Client) CreateSign(req *upstream.CreateSignRequest) (*upstream.CreateSignResponse, error) {
	clean := func(s string) string { return strings.TrimSpace(strings.Trim(s, "`")) }

	payload := map[string]interface{}{
		"sign_name":        req.Content,
		"label":            req.Label,
		"credit_code_url":  clean(req.CreditCodeURL),
		"id_card_front":    clean(req.IDCardFront),
		"id_card_back":     clean(req.IDCardBack),
		"company":          req.Company,
		"legal_person":     req.LegalPerson,
		"credit_code":      req.CreditCode,
		"credit_user_name": req.CreditUserName,
		"id_card":          req.IDCard,
		"phone":            req.Phone,
		"auth_letter":      clean(req.AuthLetter),
	}
	if v := clean(req.SxCommits); v != "" {
		payload["sx_commits"] = v
	}
	if v := clean(req.Screenshot); v != "" {
		payload["screenshot"] = v
	}

	var out struct {
		SignID   int64  `json:"sign_id"`
		UpSignID string `json:"up_sign_id"`
		Status   int    `json:"status"`
	}
	if err := c.call(http.MethodPost, "/v1/sms/signs", "sms-sign-create", "", payload, &out); err != nil {
		return nil, err
	}
	if out.SignID <= 0 {
		return nil, fmt.Errorf("StarLoft 平台未返回签名 ID")
	}
	return &upstream.CreateSignResponse{SignId: out.SignID, Status: "00", Message: "success"}, nil
}

// DeleteSign 删除短信签名：DELETE /v1/sms/signs/:id（signID 为平台签名记录主键）
func (c *Client) DeleteSign(signID string) error {
	if strings.TrimSpace(signID) == "" {
		return fmt.Errorf("签名ID不能为空")
	}
	return c.call(http.MethodDelete, "/v1/sms/signs/"+url.PathEscape(strings.TrimSpace(signID)), "sms-sign-delete", signID, nil, nil)
}

// CreateFreeTemplate 报备短信模板：POST /v1/sms/templates。
// 返回平台模板标识（优先上游模板 ID），业务层以其作为本地模板的「上游模板 ID」。
func (c *Client) CreateFreeTemplate(req *upstream.CreateTemplateRequest) (string, error) {
	payload := map[string]interface{}{
		"title":    req.TemplateName,
		"content":  req.Content,
		"sms_type": req.SmsType,
		"sign_id":  req.SignId,
	}
	var out struct {
		Template struct {
			TemplateID     string `json:"template_id"`
			TemplateStatus int    `json:"template_status"`
		} `json:"template"`
	}
	if err := c.call(http.MethodPost, "/v1/sms/templates", "sms-template-create", "", payload, &out); err != nil {
		return "", err
	}
	if out.Template.TemplateID == "" {
		return "", fmt.Errorf("StarLoft 平台未返回模板 ID")
	}
	return out.Template.TemplateID, nil
}

// UpdateTemplate 修改短信模板：PUT /v1/sms/templates/:id（平台仅支持改内容，名称与绑定签名不随之上送）。
func (c *Client) UpdateTemplate(templateID string, signID int64, name, content, smsType string) error {
	if strings.TrimSpace(templateID) == "" {
		return fmt.Errorf("模板ID不能为空")
	}
	return c.call(http.MethodPut, "/v1/sms/templates/"+url.PathEscape(strings.TrimSpace(templateID)),
		"sms-template-update", templateID, map[string]interface{}{"content": content}, nil)
}

// GetSignStatus 查询签名审核状态：GET /v1/sms/signs/:id。
// 平台本地状态（0-待审核 1-审核中 2-通过 3-驳回）归一为上游状态码空间
// （1-通过 2-待审核 3-驳回），供业务层按同一套映射回写本地。
func (c *Client) GetSignStatus(signID string) (int, string, error) {
	if strings.TrimSpace(signID) == "" {
		return 0, "", fmt.Errorf("签名ID不能为空")
	}
	var out struct {
		SignID        int64  `json:"sign_id"`
		UpSignID      string `json:"up_sign_id"`
		Status        int    `json:"status"`
		ResultMessage string `json:"result_message"`
	}
	if err := c.call(http.MethodGet, "/v1/sms/signs/"+url.PathEscape(strings.TrimSpace(signID)), "sms-sign-get", signID, nil, &out); err != nil {
		return 0, "", err
	}
	switch out.Status {
	case 2:
		return upstreamStatusApproved, "", nil
	case 3:
		return upstreamStatusRejected, out.ResultMessage, nil
	default:
		return upstreamStatusPending, "", nil
	}
}

// GetTemplateStatus 查询模板审核状态：GET /v1/sms/templates/:id。
// 平台模板状态（1-审核中 2-通过 3-未通过）归一为上游状态码空间（1-通过 2-待审核 3-驳回）。
func (c *Client) GetTemplateStatus(templateID, smsType string) (int, string, error) {
	if strings.TrimSpace(templateID) == "" {
		return 0, "", fmt.Errorf("模板ID不能为空")
	}
	var out struct {
		Template struct {
			TemplateID     string `json:"template_id"`
			TemplateStatus int    `json:"template_status"`
			Msg            string `json:"msg"`
		} `json:"template"`
	}
	if err := c.call(http.MethodGet, "/v1/sms/templates/"+url.PathEscape(strings.TrimSpace(templateID)),
		"sms-template-get", templateID, nil, &out); err != nil {
		return 0, "", err
	}
	switch out.Template.TemplateStatus {
	case 2:
		return upstreamStatusApproved, "", nil
	case 3:
		return upstreamStatusRejected, out.Template.Msg, nil
	default:
		return upstreamStatusPending, "", nil
	}
}

// PullReport 拉取短信发送回执：POST /v1/sms/report（按平台任务号分页拉取）
func (c *Client) PullReport(taskId string, pageNo, pageSize int) ([]upstream.SmsReportItem, error) {
	if strings.TrimSpace(taskId) == "" {
		return nil, fmt.Errorf("taskId 不能为空")
	}
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	payload := map[string]interface{}{"task_id": taskId, "page_no": pageNo, "page_size": pageSize}
	var out struct {
		TaskID  string                    `json:"task_id"`
		Reports []upstream.SmsReportItem   `json:"reports"`
	}
	if err := c.call(http.MethodPost, "/v1/sms/report", "sms-report", taskId, payload, &out); err != nil {
		return nil, err
	}
	return out.Reports, nil
}

// PullReply 拉取短信上行回复：POST /v1/sms/replies（按日期 yyyyMMdd 向上游拉取并同步落库后返回）
func (c *Client) PullReply(date string, pageNo, pageSize int) ([]upstream.SmsReplyItem, error) {
	if strings.TrimSpace(date) == "" {
		return nil, fmt.Errorf("回复日期不能为空")
	}
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	if pageSize > 100 {
		pageSize = 100
	}
	payload := map[string]interface{}{"date": date, "page_no": pageNo, "page_size": pageSize}
	var out struct {
		Total   int `json:"total"`
		Replies []struct {
			TaskID      string `json:"task_id"`
			Phone       string `json:"phone"`
			SequenceID  string `json:"sequence_id"`
			ContentDown string `json:"content_down"`
			ContentUp   string `json:"content_up"`
			RespTime    string `json:"resp_time"`
			Status      string `json:"status"`
			Tag         string `json:"tag"`
		} `json:"replies"`
	}
	if err := c.call(http.MethodPost, "/v1/sms/replies", "sms-replies", date, payload, &out); err != nil {
		return nil, err
	}

	items := make([]upstream.SmsReplyItem, 0, len(out.Replies))
	for _, r := range out.Replies {
		items = append(items, upstream.SmsReplyItem{
			TaskId:      r.TaskID,
			Phone:       r.Phone,
			SequenceId:  r.SequenceID,
			ContentDown: r.ContentDown,
			ContentUp:   r.ContentUp,
			RespTime:    r.RespTime,
			Status:      r.Status,
			Tag:         r.Tag,
		})
	}
	return items, nil
}

