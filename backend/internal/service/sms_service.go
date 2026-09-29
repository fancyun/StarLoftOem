package service

import (
	"encoding/json"
	"fmt"

	"oemrpa/internal/upstream"
)

// SMSService 短信服务（平台自用验证码短信，腾讯云通道）
type SMSService struct {
	tencent       *upstream.TencentSmsClient
	verifySign    string // 平台验证码短信签名（已审核）
	verifyTplID   string // 平台验证码短信模板ID（已审核）
	verifyCodeSvc *VerificationCodeService
}

// NewSMSService 创建短信服务
// 未配置腾讯云短信通道或平台验证码签名时返回 nil（不启用短信验证码功能）
func NewSMSService(tencent *upstream.TencentSmsClient, verifySign, verifyTplID string) *SMSService {
	if tencent == nil || verifySign == "" {
		return nil
	}
	return &SMSService{
		tencent:       tencent,
		verifySign:    verifySign,
		verifyTplID:   verifyTplID,
		verifyCodeSvc: NewVerificationCodeService(),
	}
}

// Enabled 短信服务是否可用
func (s *SMSService) Enabled() bool {
	return s != nil && s.tencent != nil
}

// SendVerificationCode 发送验证码（腾讯云短信通道）
func (s *SMSService) SendVerificationCode(phone string) error {
	if !s.Enabled() {
		return fmt.Errorf("短信服务未配置")
	}

	// 检查发送频率限制
	canSend, remainSeconds, err := s.verifyCodeSvc.CheckSendRateLimit(phone)
	if err != nil {
		return fmt.Errorf("检查发送频率失败: %w", err)
	}
	if !canSend {
		return fmt.Errorf("发送过于频繁，请 %d 秒后再试", remainSeconds)
	}

	// 生成验证码
	code := s.verifyCodeSvc.GenerateCode()

	// 集合类字段序列化为 JSON 字符串随 body 上送（参与签名黑名单，不上送签名）
	phoneSetJSON, _ := json.Marshal([]string{phone})
	// 模板参数与平台验证码短信模板一致（验证码）（腾讯云模板：您的验证码为{1}，5分钟内有效）
	paramSetJSON, _ := json.Marshal([]string{code})

	resp, err := s.tencent.SendSms(&upstream.SendSmsRequest{
		PhoneNumberSet:   string(phoneSetJSON),
		TemplateId:       s.verifyTplID,
		SignName:         s.verifySign,
		TemplateParamSet: string(paramSetJSON),
	})
	if err != nil {
		return fmt.Errorf("发送短信失败: %w", err)
	}
	if resp.Status != "" && resp.Status != "00" {
		return fmt.Errorf("短信发送失败: status=%s msg=%s", resp.Status, resp.Message)
	}

	// 保存验证码到 Redis
	if err := s.verifyCodeSvc.SaveCode(phone, code); err != nil {
		return fmt.Errorf("保存验证码失败: %w", err)
	}

	// 设置发送频率限制
	if err := s.verifyCodeSvc.SetSendRateLimit(phone); err != nil {
		return fmt.Errorf("设置频率限制失败: %w", err)
	}

	return nil
}

// VerifyCode 验证验证码
func (s *SMSService) VerifyCode(phone, code string) (bool, error) {
	return s.verifyCodeSvc.VerifyCode(phone, code)
}
