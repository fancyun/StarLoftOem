package upstream

import (
	"strconv"
	"strings"
)

// FlexInt 兼容上游返回的 result_code：可能是字符串（如 "1000"）也可能是数字。
type FlexInt int

// UnmarshalJSON 同时支持字符串和数字两种 JSON 表示。
func (f *FlexInt) UnmarshalJSON(data []byte) error {
	s := strings.TrimSpace(string(data))
	s = strings.Trim(s, `"`)
	if s == "" || s == "null" {
		*f = 0
		return nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return err
	}
	*f = FlexInt(n)
	return nil
}

// ========== 请求/响应结构体（对齐 H5 Plus 文档） ==========

// GetTokenRequest get_token 请求参数（multipart/form-data）
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_plus_get_token
type GetTokenRequest struct {
	// 必选参数
	Sign           string // HMAC 签名
	SignVersion    string // 签名算法版本: hmac_sha1 / hmac_sha256
	ReturnURL      string // 验证完成后跳转URL
	NotifyURL      string // 回调URL
	BizNo          string // 客户业务流水号，唯一，不超过128字节
	SceneID        string // 场景ID
	ComparisonType string // "1"=人脸核验模式
	UUID           string // 用户唯一标识，不超过512字节

	// 身份证拍摄模式参数
	IDCardMode   string // "0"=不拍摄, "1"=仅正面, "2"=正反面
	IDCardName   string // idcard_mode=0时使用，可为空
	IDCardNumber string // idcard_mode=0时使用，可为空

	// 可选参数
	BizExtraData   string // 额外数据，不超过4096字节
	EncryptionType string // 加密类型: 0/1/2
}

// GetTokenResponse get_token 响应
type GetTokenResponse struct {
	RequestID    string `json:"request_id"`
	TimeUsed     int    `json:"time_used"`
	Token        string `json:"token"`
	BizID        string `json:"biz_id"`
	ExpiredTime  int64  `json:"expired_time"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// GetResultRequest get_result 请求参数（GET 请求，Query 参数）
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_plus_get_result
type GetResultRequest struct {
	BizID       string // 业务流串号，必选
	Sign        string // HMAC 签名，必选
	SignVersion string // 签名算法版本，必选
}

// GetResultResponse get_result 响应
type GetResultResponse struct {
	RequestID     string  `json:"request_id"`
	TimeUsed      int     `json:"time_used"`
	ResultCode    FlexInt `json:"result_code"`
	ResultMessage string  `json:"result_message"`
	BizInfo       struct {
		BizID        string `json:"biz_id"`
		BizNo        string `json:"biz_no"`
		BizExtraData string `json:"biz_extra_data"`
	} `json:"biz_info"`
	LivenessResult map[string]interface{}   `json:"liveness_result,omitempty"`
	VerifyResult   map[string]interface{}   `json:"verify_result,omitempty"`
	WillResult     []map[string]interface{} `json:"will_result,omitempty"`
	Images         map[string]string        `json:"images,omitempty"`
	Video          string                   `json:"video,omitempty"`
	VerifyRiskInfo map[string]interface{}   `json:"verify_risk_info,omitempty"`
	DeviceRiskInfo map[string]interface{}   `json:"device_risk_info,omitempty"`
	ErrorMessage   string                   `json:"error_message,omitempty"`
}

// NotifyData 异步回调数据（notify_url / return_url 回调）
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_plus_return_notify_url
type NotifyData struct {
	RequestID     string  `json:"request_id"`
	TimeUsed      int     `json:"time_used"`
	ResultCode    FlexInt `json:"result_code"`
	ResultMessage string  `json:"result_message"`
	BizInfo       struct {
		BizID        string `json:"biz_id"`
		BizNo        string `json:"biz_no"`
		BizExtraData string `json:"biz_extra_data"`
	} `json:"biz_info"`
	LivenessResult map[string]interface{}   `json:"liveness_result,omitempty"`
	VerifyResult   map[string]interface{}   `json:"verify_result,omitempty"`
	WillResult     []map[string]interface{} `json:"will_result,omitempty"`
	Images         map[string]string        `json:"images,omitempty"`
	Video          string                   `json:"video,omitempty"`
	VerifyRiskInfo map[string]interface{}   `json:"verify_risk_info,omitempty"`
	DeviceRiskInfo map[string]interface{}   `json:"device_risk_info,omitempty"`
}
