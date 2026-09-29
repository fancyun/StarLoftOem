package upstream

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"oemrpa/internal/logstore"
)

// ShlianluClient 联麓信息（shlianlu）短信网关客户端（短信 API 3.0）。
// 全部接口为 POST + JSON；发送仅支持模板短信（/sms/trade/template/send，Type=3），
// 签名报备走 /sms/product/sign/create，模板报备走 /sms/product/template/create。
type ShlianluClient struct {
	baseURL        string // API 基址（如 https://apis.shlianlu.com）
	mchID          string // 企业ID（MchId）
	appID          string // 应用ID（AppId，验证码/通知短信通道）
	marketingAppID string // 应用ID（AppId，营销短信通道；空表示营销不可用）
	key            string // AppKey（MD5 签名密钥）
	client         *http.Client
}

// shlianluTemplateQueryBaseURL 模板查询接口所在域名（与其余接口不同：api.shlianlu.com，见联麓文档 4.2）
const shlianluTemplateQueryBaseURL = "https://api.shlianlu.com"

// NewShlianluClient 创建联麓短信网关客户端
func NewShlianluClient(baseURL, mchID, appID, marketingAppID, key string) *ShlianluClient {
	return &ShlianluClient{
		baseURL:        strings.TrimRight(baseURL, "/"),
		mchID:          mchID,
		appID:          appID,
		marketingAppID: marketingAppID,
		key:            key,
		client:         &http.Client{Timeout: 30 * time.Second},
	}
}

// MarketingAvailable 营销短信通道是否可用（未配置营销 AppId 时不可用）
func (c *ShlianluClient) MarketingAvailable() bool {
	return c.marketingAppID != ""
}

// appIDForSmsType 按短信类型选应用：营销短信走营销 AppId，其余走通知 AppId
func (c *ShlianluClient) appIDForSmsType(smsType string) string {
	if smsType == SmsTypeMarketing {
		return c.marketingAppID
	}
	return c.appID
}

// signIgnoreFields 不参与签名的参数（集合类字段与签名字段本身）
var signIgnoreFields = map[string]struct{}{
	"PhoneNumberSet":    {},
	"SessionContext":    {},
	"SessionContextSet": {},
	"ContextParamSet":   {},
	"TemplateParamSet":  {},
	"Signature":         {},
	"PhoneList":         {},
	"phoneSet":          {},
}

// sign 计算 MD5 签名：参数名排序 → 剔除集合类与空值 → k=v&... 拼接 → 末尾追加 key=AppKey → MD5 转大写
func (c *ShlianluClient) sign(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if v == "" {
			continue
		}
		if _, ok := signIgnoreFields[k]; ok {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
	}
	sb.WriteString("&key=")
	sb.WriteString(c.key)

	sum := md5.Sum([]byte(sb.String()))
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

// postJSON 调用联麓 POST 接口（默认基址与通知 AppId）：注入公共参数
// （MchId/AppId/Version/SignType/TimeStamp）并计算签名。
// body 为业务参数（int/string 均支持，集合类字段不参与签名，由 sign 内黑名单剔除）。
func (c *ShlianluClient) postJSON(path, version string, body map[string]interface{}) ([]byte, error) {
	return c.postJSONWithAppID(c.appID, c.baseURL, path, version, body)
}

// postJSONForSmsType 同 postJSON，但按短信类型选应用（营销短信走营销 AppId）
func (c *ShlianluClient) postJSONForSmsType(smsType, path, version string, body map[string]interface{}) ([]byte, error) {
	return c.postJSONWithAppID(c.appIDForSmsType(smsType), c.baseURL, path, version, body)
}

// postJSONWithAppID 调用联麓 POST 接口并指定 AppId 与基址（联麓以 AppId 区分验证码/通知与营销通道；
// 模板查询接口域名与其余不同，见 shlianluTemplateQueryBaseURL）
func (c *ShlianluClient) postJSONWithAppID(appID, baseURL, path, version string, body map[string]interface{}) ([]byte, error) {
	// 注入公共参数（TimeStamp 为 UNIX 秒级时间戳）
	body["MchId"] = c.mchID
	body["AppId"] = appID
	body["Version"] = version
	body["SignType"] = "MD5"
	body["TimeStamp"] = strconv.FormatInt(time.Now().Unix(), 10)

	// 计算签名：值统一转字符串（数字与字符串等价参与签名）
	signParams := make(map[string]string, len(body))
	for k, v := range body {
		signParams[k] = fmt.Sprintf("%v", v)
	}
	body["Signature"] = c.sign(signParams)

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("序列化 shlianlu 请求失败: %w", err)
	}

	req, err := http.NewRequest("POST", baseURL+path, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("shlianlu http %d: %s", resp.StatusCode, string(out))
	}
	return out, nil
}

// shlianluResponse 联麓接口统一响应结构（业务字段按接口各取所需）
type shlianluResponse struct {
	Status     string `json:"status"`     // "00" 成功，其余见错误码
	Message    string `json:"message"`    // 返回信息
	Timestamp  int64  `json:"timestamp"`  // UNIX 时间戳
	SignId     int64  `json:"SignId"`     // 创建签名返回
	TemplateId int64  `json:"TemplateId"` // 创建模板返回
	TaskId     string `json:"taskId"`     // 发送返回的唯一请求 ID
	Count      int    `json:"count"`      // 发送预扣费条数
}

// SendSms 模板短信发送。
// 联麓 3.0 无「直发完整内容」接口，发送必须带 TemplateId；req.Content 直发一律拒绝。
// req.SmsType 为该次发送所绑定模板的类型（由业务侧按模板解析后回填）：营销走营销 AppId，其余走通知 AppId。
func (c *ShlianluClient) SendSms(req *SendSmsRequest) (*SendSmsResponse, error) {
	if req.TemplateId == "" {
		return nil, fmt.Errorf("shlianlu 仅支持模板发送，请先创建并审核通过短信模板")
	}
	if req.Content != "" {
		return nil, fmt.Errorf("shlianlu 仅支持模板发送，不支持直接发送短信内容")
	}
	var phones, params []string
	if err := json.Unmarshal([]byte(req.PhoneNumberSet), &phones); err != nil || len(phones) == 0 {
		return nil, fmt.Errorf("手机号码不能为空")
	}
	_ = json.Unmarshal([]byte(req.TemplateParamSet), &params)
	if params == nil {
		params = []string{}
	}

	respBody, err := c.postJSONForSmsType(req.SmsType, "/sms/trade/template/send", "1.2.0", map[string]interface{}{
		"Type":             "3",
		"TemplateId":       req.TemplateId,
		"PhoneNumberSet":   phones,
		"TemplateParamSet": params,
	})
	if err != nil {
		return nil, err
	}

	var out shlianluResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("解析 shlianlu 响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu", req.SessionContext, 0, "", "", string(respBody), callStatus)

	return &SendSmsResponse{
		TaskId:     out.TaskId,
		Status:     out.Status,
		Message:    out.Message,
		MessageSid: out.TaskId,
		Count:      out.Count,
	}, nil
}

// CreateSign 创建短信签名（实现 SmsSignChannel）：映射联麓 sign/create 报备
func (c *ShlianluClient) CreateSign(req *CreateSignRequest) (*CreateSignResponse, error) {
	// 兜底清理 URL 字段两端反引号/空白（前端可能在图片地址两端误带入反引号，导致上游判为非法 URL 报「材料不能为空」）
	clean := func(s string) string { return strings.TrimSpace(strings.Trim(s, "`")) }
	req.CreditCodeURL = clean(req.CreditCodeURL)
	req.IDCardFront = clean(req.IDCardFront)
	req.IDCardBack = clean(req.IDCardBack)
	req.AuthLetter = clean(req.AuthLetter)
	req.Screenshot = clean(req.Screenshot)
	req.SxCommits = clean(req.SxCommits)

	// 签名在通知与营销通道间共用，意愿承诺函非必传：未填写时以营业执照 URL 兜底，避免上游必填校验报错
	sxCommits := req.SxCommits
	if sxCommits == "" {
		sxCommits = req.CreditCodeURL
	}
	body := map[string]interface{}{
		"content": req.Content,
		"type":    req.SignType, // 默认 2（他公司），由调用方兜底
		"label":   req.Label,    // 默认 1（营业执照），由调用方兜底
	}
	for k, v := range map[string]string{
		"creditCodeUrl":  req.CreditCodeURL,
		"idCardFront":    req.IDCardFront,
		"idCardBack":     req.IDCardBack,
		"sxCommits":      sxCommits,
		"contract":       req.AuthLetter, // 联麓字段名：签名公司的授权书（type=2 必填）
		"screenshot":     req.Screenshot,
		"company":        req.Company,
		"legalPerson":    req.LegalPerson,
		"creditCode":     req.CreditCode,
		"creditUserName": req.CreditUserName,
		"idCard":         req.IDCard,
		"phone":          req.Phone,
	} {
		if v != "" {
			body[k] = v
		}
	}

	respBody, err := c.postJSON("/sms/product/sign/create", "1.2.0", body)
	if err != nil {
		return nil, err
	}

	var out shlianluResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("解析 shlianlu 响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	// 记录本次请求体（含资质 URL），便于排查上游「材料不能为空」类校验
	reqBody, _ := json.Marshal(body)
	logstore.RecordSysCall("request", "shlianlu", "", 0, "", string(reqBody), string(respBody), callStatus)

	return &CreateSignResponse{
		SignId:  out.SignId,
		Message: out.Message,
		Status:  out.Status,
	}, nil
}

// DeleteSign 删除短信签名（修改签名时，先创建新签名再删除旧签名）。
// 上游 sms/product/sign/delete（文档 §3.3）。返回删除是否成功（上游 status=="00" 或提示不存在均视为成功，便于幂等）。
func (c *ShlianluClient) DeleteSign(signID string) error {
	id, err := strconv.ParseInt(signID, 10, 64)
	if err != nil {
		return fmt.Errorf("签名ID非法: %s", signID)
	}
	respBody, err := c.postJSON("/sms/product/sign/delete", "1.2.0", map[string]interface{}{
		"SignId": id,
	})
	if err != nil {
		return err
	}
	var out shlianluResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return fmt.Errorf("解析 shlianlu 删除签名响应失败: %w", err)
	}
	callStatus := 1
	// status=24（签名ID不存在）视为已删除，保证「删除旧签名→新建」流程可重复执行
	if out.Status != "00" && out.Status != "0" && out.Status != "24" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu-sign-delete", signID, 0, "", "", string(respBody), callStatus)
	if out.Status != "00" && out.Status != "0" && out.Status != "24" {
		return fmt.Errorf("shlianlu 删除签名失败: status=%s msg=%s", out.Status, out.Message)
	}
	return nil
}

// CreateFreeTemplate 创建短信模板（实现 SmsTemplateChannel）：映射联麓 template/create，返回 TemplateId。
// req.SmsType 为模板类型：营销模板须在营销 AppId 下报备（上游按 AppId 区分通道），其余走通知 AppId。
func (c *ShlianluClient) CreateFreeTemplate(req *CreateTemplateRequest) (string, error) {
	respBody, err := c.postJSONForSmsType(req.SmsType, "/sms/product/template/create", "1.2.0", map[string]interface{}{
		"SignId":       req.SignId,
		"TemplateName": req.TemplateName,
		"content":      req.Content,
	})
	if err != nil {
		return "", err
	}

	var out shlianluResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("解析 shlianlu 响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu", "", 0, "", "", string(respBody), callStatus)
	if out.Status != "00" {
		return "", fmt.Errorf("shlianlu 模板报备失败: status=%s msg=%s", out.Status, out.Message)
	}
	return strconv.FormatInt(out.TemplateId, 10), nil
}

// UpdateTemplate 编辑短信模板：映射联麓 template/update（TemplateId 必填；名称/内容/签名按需传）。
// signID > 0 时同时更新模板绑定的签名；smsType 为模板类型（营销模板走营销 AppId 编辑）。
func (c *ShlianluClient) UpdateTemplate(templateID string, signID int64, name, content, smsType string) error {
	id, err := strconv.ParseInt(templateID, 10, 64)
	if err != nil {
		return fmt.Errorf("模板ID非法: %s", templateID)
	}
	body := map[string]interface{}{
		"TemplateId":   id,
		"TemplateName": name,
		"content":      content,
	}
	if signID > 0 {
		body["SignId"] = signID
	}
	respBody, err := c.postJSONForSmsType(smsType, "/sms/product/template/update", "1.2.0", body)
	if err != nil {
		return err
	}

	var out shlianluResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		return fmt.Errorf("解析 shlianlu 响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu", "", 0, "", "", string(respBody), callStatus)
	if out.Status != "00" {
		return fmt.Errorf("shlianlu 模板编辑失败: status=%s msg=%s", out.Status, out.Message)
	}
	return nil
}

// PullReport 拉取发送报告（按 taskId）：上游 sms/trade/report（Version 1.2.0）
func (c *ShlianluClient) PullReport(taskId string, pageNo, pageSize int) ([]SmsReportItem, error) {
	if taskId == "" {
		return nil, fmt.Errorf("taskId 不能为空")
	}
	if pageNo <= 0 {
		pageNo = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	respBody, err := c.postJSON("/sms/trade/report", "1.2.0", map[string]interface{}{
		"TaskId":   taskId,
		"pageNo":   pageNo,
		"pageSize": pageSize,
	})
	if err != nil {
		return nil, err
	}

	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    []struct {
			SequenceId string `json:"sequenceId"`
			Phone      string `json:"phone"`
			Content    string `json:"content"`
			Status     string `json:"status"`
			RespTime   string `json:"respTime"`
			RespCode   string `json:"respCode"`
			CodeDesc   string `json:"codeDesc"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("解析 shlianlu 回执响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu-report", taskId, 0, "", "", string(respBody), callStatus)
	if out.Status != "00" {
		return nil, fmt.Errorf("shlianlu 拉取报告失败: status=%s msg=%s", out.Status, out.Message)
	}

	items := make([]SmsReportItem, 0, len(out.Data))
	for _, d := range out.Data {
		items = append(items, SmsReportItem{
			SequenceId: d.SequenceId,
			Phone:      d.Phone,
			Content:    d.Content,
			Status:     d.Status,
			RespTime:   d.RespTime,
			RespCode:   d.RespCode,
			CodeDesc:   d.CodeDesc,
		})
	}
	return items, nil
}

// PullReply 拉取短信上行回复（实现 SmsReplyChannel）：上游 sms/trade/reply（Version 1.2.0）。
// 按日期拉取（格式 yyyyMMdd）；上游对已拉取回复标记已读，再次拉取不再展示。
func (c *ShlianluClient) PullReply(date string, pageNo, pageSize int) ([]SmsReplyItem, error) {
	if date == "" {
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
	respBody, err := c.postJSON("/sms/trade/reply", "1.2.0", map[string]interface{}{
		"Date":     date,
		"pageNo":   pageNo,
		"pageSize": pageSize,
	})
	if err != nil {
		return nil, err
	}

	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    []struct {
			TaskId      string `json:"taskId"`
			Phone       string `json:"phone"`
			RespTime    string `json:"respTime"`
			RespContent string `json:"respContent"`
			Tag         string `json:"tag"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, fmt.Errorf("解析 shlianlu 回复响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu-reply", date, 0, "", "", string(respBody), callStatus)
	if out.Status != "00" {
		return nil, fmt.Errorf("shlianlu 拉取回复失败: status=%s msg=%s", out.Status, out.Message)
	}

	items := make([]SmsReplyItem, 0, len(out.Data))
	for _, d := range out.Data {
		items = append(items, SmsReplyItem{
			TaskId:      d.TaskId,
			Phone:       d.Phone,
			RespTime:    d.RespTime,
			RespContent: d.RespContent,
			Tag:         d.Tag,
		})
	}
	return items, nil
}

// GetSignStatus 查询签名审核状态（实现 SmsSignStatusChannel）：上游 sms/product/sign/get。
// 返回上游状态（1-审核通过 2-待审核 3-审核不通过 4-报备中）与驳回原因。
func (c *ShlianluClient) GetSignStatus(signID string) (int, string, error) {
	id, err := strconv.ParseInt(signID, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("签名ID非法: %s", signID)
	}
	respBody, err := c.postJSON("/sms/product/sign/get", "1.2.0", map[string]interface{}{
		"SignId": id,
	})
	if err != nil {
		return 0, "", err
	}

	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    []struct {
			SignId       int64  `json:"signId"`
			Content      string `json:"content"`
			Status       int    `json:"status"`
			RefuseReason string `json:"refuseReason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return 0, "", fmt.Errorf("解析 shlianlu 签名查询响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu-sign-get", signID, 0, "", "", string(respBody), callStatus)
	if out.Status != "00" {
		return 0, "", fmt.Errorf("shlianlu 签名查询失败: status=%s msg=%s", out.Status, out.Message)
	}
	if len(out.Data) == 0 {
		return 0, "", fmt.Errorf("上游未查询到该签名")
	}
	return out.Data[0].Status, out.Data[0].RefuseReason, nil
}

// GetTemplateStatus 查询模板审核状态（实现 SmsTemplateStatusChannel）：
// 上游 sms/product/template/getById（注意该接口域名为 api.shlianlu.com）。
// smsType 为模板类型（营销模板须按营销 AppId 查询，上游按 AppId 区分通道）。
// 返回上游状态（1-审核通过 2-待审核 3-审核驳回）与驳回原因。
func (c *ShlianluClient) GetTemplateStatus(templateID, smsType string) (int, string, error) {
	id, err := strconv.ParseInt(templateID, 10, 64)
	if err != nil {
		return 0, "", fmt.Errorf("模板ID非法: %s", templateID)
	}
	respBody, err := c.postJSONWithAppID(c.appIDForSmsType(smsType), shlianluTemplateQueryBaseURL, "/sms/product/template/getById", "1.2.0", map[string]interface{}{
		"TemplateId": id,
	})
	if err != nil {
		return 0, "", err
	}

	var out struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Data    struct {
			ID           int64  `json:"id"`
			Status       int    `json:"status"`
			RefuseReason string `json:"refuseReason"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return 0, "", fmt.Errorf("解析 shlianlu 模板查询响应失败: %w", err)
	}
	callStatus := 1
	if out.Status != "00" {
		callStatus = 0
	}
	logstore.RecordSysCall("request", "shlianlu-template-get", templateID, 0, "", "", string(respBody), callStatus)
	if out.Status != "00" {
		return 0, "", fmt.Errorf("shlianlu 模板查询失败: status=%s msg=%s", out.Status, out.Message)
	}
	return out.Data.Status, out.Data.RefuseReason, nil
}
