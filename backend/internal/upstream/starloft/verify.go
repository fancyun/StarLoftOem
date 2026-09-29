package starloft

import (
	"fmt"
	"net/http"
	"strings"
)

// SendVerificationSms 下发平台验证码短信：POST /v1/sms/send（模板参数为验证码本身）。
// 不传 notify_url：验证码短信用途单一、无需回执回调，避免无谓的下游推送。
func (c *Client) SendVerificationSms(phone, templateID, signName, code string) error {
	if strings.TrimSpace(phone) == "" || strings.TrimSpace(templateID) == "" {
		return fmt.Errorf("手机号或验证码模板未配置")
	}
	payload := map[string]interface{}{
		"phone_number_set": []string{phone},
		"template_id":      templateID,
		"template_params":  []string{code},
		"sms_type":         "verify",
	}
	if signName != "" {
		payload["sign_name"] = signName
	}
	return c.call(http.MethodPost, "/v1/sms/send", "sms-verify", "", payload, nil)
}