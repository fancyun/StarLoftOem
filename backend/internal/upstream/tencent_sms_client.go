package upstream

import (
	"encoding/json"
	"fmt"
)

// TencentSmsClient 腾讯云短信客户端（平台自用验证码）。
// 平台验证码使用的签名/模板在腾讯云短信控制台创建，凭据由 .env 配置。
type TencentSmsClient struct {
	client   *TencentClient
	sdkAppID string // 短信应用 SDKAppID（字符串）
}

// NewTencentSmsClient 创建腾讯云短信客户端。
func NewTencentSmsClient(secretID, secretKey, region, sdkAppID string) *TencentSmsClient {
	return &TencentSmsClient{
		client:   NewTencentClient(secretID, secretKey, region),
		sdkAppID: sdkAppID,
	}
}

// Name 返回通道唯一标识（发送记录落库用）。
func (c *TencentSmsClient) Name() string { return "tencent" }

// SendSms 调用腾讯云短信 SendSms（API 3.0，sms/2021-01-11）。
// 使用 req 中已由平台渲染/指定的号码、签名、模板与参数。
func (c *TencentSmsClient) SendSms(req *SendSmsRequest) (*SendSmsResponse, error) {
	var phones, params []string
	_ = json.Unmarshal([]byte(req.PhoneNumberSet), &phones)
	_ = json.Unmarshal([]byte(req.TemplateParamSet), &params)
	if len(phones) == 0 {
		return nil, fmt.Errorf("手机号为空")
	}
	if c.sdkAppID == "" {
		return nil, fmt.Errorf("短信 SDKAppID 未配置")
	}

	respBody, err := c.client.Call("sms", "SendSms", "2021-01-11", map[string]interface{}{
		"PhoneNumberSet":   phones,
		"SmsSdkAppId":      c.sdkAppID,
		"SignName":         req.SignName,
		"TemplateId":       req.TemplateId,
		"TemplateParamSet": params,
		"SessionContext":   req.SessionContext,
	})
	if err != nil {
		return nil, err
	}
	if err := TencentError(respBody); err != nil {
		return nil, err
	}

	var r struct {
		Response struct {
			SendStatusSet []struct {
				Code      string `json:"Code"`
				Message   string `json:"Message"`
				SerialNo  string `json:"SerialNo"`
				ErrorCode string `json:"ErrorCode"`
			} `json:"SendStatusSet"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}
	if len(r.Response.SendStatusSet) == 0 {
		return nil, fmt.Errorf("腾讯云短信无返回状态")
	}
	first := r.Response.SendStatusSet[0]
	if first.Code != "Ok" {
		return nil, fmt.Errorf("发送短信失败: code=%s msg=%s", first.Code, first.Message)
	}
	return &SendSmsResponse{
		TaskId:  first.SerialNo,
		Status:  "00",
		Message: first.Message,
	}, nil
}
