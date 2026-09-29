package upstream

// 短信上游公共 DTO：各短信通道客户端（如 shlianlu / tencent）统一使用，换取通道抽象的解耦。

// SmsTypeMarketing 营销短信类型（与平台短信模板类型一致）：仅该类型需走独立应用通道与独立资源包。
const SmsTypeMarketing = "marketing"

// SendSmsRequest 发送短信请求（通道无关）。
// 集合类字段（PhoneNumberSet/TemplateParamSet）为 JSON 字符串；
// 模板型通道（shlianlu / tencent）使用 TemplateId + TemplateParamSet，
// Content 为平台渲染后的完整短信内容，直发型通道使用（当前无直发型通道，模板型通道忽略）。
type SendSmsRequest struct {
	PhoneNumberSet   string `json:"PhoneNumberSet"`             // 号码数组 JSON
	TemplateId       string `json:"TemplateId,omitempty"`       // 模板ID（模板型通道使用）
	SignName         string `json:"TemplateSign,omitempty"`     // 已审核签名
	TemplateParamSet string `json:"TemplateParamSet,omitempty"` // 模板参数数组 JSON
	Content          string `json:"Content,omitempty"`          // 直发型通道使用的完整短信内容（当前无直发型通道，模板型通道忽略）
	SessionContext   string `json:"SessionContext,omitempty"`   // 下行上下文（透传）
	SmsType          string `json:"SmsType,omitempty"`          // 短信类型（所绑定模板的类型，决定所走的应用通道）
}

// SendSmsResponse 发送短信响应（通道无关）
// status=="00" 表示成功；TaskId 为上游发送任务 ID。
type SendSmsResponse struct {
	TaskId     string `json:"taskId"`
	Status     string `json:"status"`
	Message    string `json:"message"`
	MessageSid string `json:"MessageSid"`
	Count      int    `json:"count"` // 上游预扣费条数（长短信按分条计费，平台据此校正计费）
}

// CreateSignRequest 申请短信签名请求（实现 SmsSignChannel 的通道使用）
// 字段对齐联麓短信签名报备：type=1 本公司 / 2 他公司（他公司时 company/legalPerson/creditCode
// 与经办人信息 creditUserName/idCard/phone 必填）；label=1 营业执照 / 2 商标（上游不支持 APP 报备）。
type CreateSignRequest struct {
	Content        string `json:"content"`                  // 短信签名内容
	SignType       int    `json:"type"`                     // 签名来源：1-本公司 2-他公司（默认 1）
	Label          int    `json:"label"`                    // 资质类型：1-营业执照 2-商标（默认 1）
	CreditCodeURL  string `json:"creditCodeUrl"`            // 营业执照图片 URL（他公司必填）
	IDCardFront    string `json:"idCardFront"`              // 经办人身份证正面 URL（他公司必填）
	IDCardBack     string `json:"idCardBack"`               // 经办人身份证反面 URL（他公司必填）
	SxCommits      string `json:"sxCommits,omitempty"`      // 用户接收意愿承诺函 URL（发营销短信必传）
	AuthLetter     string `json:"authLetter,omitempty"`     // 短信签名授权书 URL（他公司签名报备材料）
	Screenshot     string `json:"screenshot,omitempty"`     // 商标备案截图 URL（label=2 必填）
	Company        string `json:"company,omitempty"`        // 公司名称（他公司必填）
	LegalPerson    string `json:"legalPerson,omitempty"`    // 法人姓名（他公司必填）
	CreditCode     string `json:"creditCode,omitempty"`     // 统一社会信用代码（他公司必填）
	CreditUserName string `json:"creditUserName,omitempty"` // 经办人姓名（他公司必填）
	IDCard         string `json:"idCard,omitempty"`         // 经办人身份证号（他公司必填）
	Phone          string `json:"phone,omitempty"`          // 经办人手机号（他公司必填）
}

// CreateSignResponse 创建签名响应
// status=="00" 表示成功；SignId 为签名 ID。
type CreateSignResponse struct {
	SignId    int64  `json:"SignId"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Status    string `json:"status"`
}

// CreateTemplateRequest 报备短信模板请求（实现 SmsTemplateChannel 的通道使用）
type CreateTemplateRequest struct {
	SignId       int64  `json:"SignId"`       // 上游签名 ID（须是审核通过的签名）
	TemplateName string `json:"TemplateName"` // 模板名称
	Content      string `json:"content"`      // 模板内容（无需携带签名）
	SmsType      string `json:"-"`            // 模板类型（营销模板在营销 AppId 下报备）
}

// SmsReportItem 短信回执明细（拉取报告 / 回执推送均用此结构，通道无关）
// respCode 为 "DELIVRD" 表示发送成功，其余为发送失败（见短信状态码表）。
type SmsReportItem struct {
	SequenceId string `json:"sequenceId"` // 序列 ID（唯一值）
	Phone      string `json:"phone"`      // 接收号码
	Content    string `json:"content"`    // 短信明细
	Status     string `json:"status"`     // 0-未知 1-发送成功 2-发送失败
	RespTime   string `json:"respTime"`   // 接收时间（状态未知为空）
	RespCode   string `json:"respCode"`   // DELIVRD 为成功，其余失败
	CodeDesc   string `json:"codeDesc"`   // 回执码中文描述
}

// SmsReplyItem 短信上行回复明细（回复推送 / 拉取回复均用此结构，通道无关）。
// 推送带 contentDown/contentUp/sequenceId/timestamp；拉取带 respTime/respContent/taskId。
type SmsReplyItem struct {
	TaskId      string `json:"taskId"`      // 上游任务 ID（关联发送记录 message_sid）
	Phone       string `json:"phone"`       // 接收号码（回复来源）
	SequenceId  string `json:"sequenceId"`  // 序列 ID（推送有，唯一）
	ContentDown string `json:"contentDown"` // 下行短信内容（推送有）
	ContentUp   string `json:"contentUp"`   // 回复内容（推送有）
	RespTime    string `json:"respTime"`    // 回复时间（拉取有）
	RespContent string `json:"respContent"` // 回复内容（拉取有）
	Timestamp   string `json:"timestamp"`   // 推送时间戳（推送有）
	Status      string `json:"status"`      // 推送状态
	Tag         string `json:"tag"`         // 自定义标签（原样回传）
}
