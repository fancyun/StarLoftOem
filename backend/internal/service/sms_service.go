package service

import (
	"fmt"
)

// VerificationSmsSender 平台自用验证码短信发送通道（OEM 系统唯一上游：StarLoft 平台开放 API）
type VerificationSmsSender interface {
	// SendVerificationSms 以已审核的模板下发一条验证码短信
	SendVerificationSms(phone, templateID, signName, code string) error
}

// SMSService 短信服务（平台自用验证码短信，经上游 StarLoft 平台下发）
type SMSService struct {
	sender        VerificationSmsSender
	verifySign    string // 平台验证码短信签名（上游已审核）
	verifyTplID   string // 平台验证码短信模板 ID（上游已审核）
	verifyCodeSvc *VerificationCodeService
}

// NewSMSService 创建短信服务；未配置上游通道、平台验证码签名或模板时返回 nil（不启用短信验证码功能）
func NewSMSService(sender VerificationSmsSender, verifySign, verifyTplID string) *SMSService {
	if sender == nil || verifySign == "" || verifyTplID == "" {
		return nil
	}
	return &SMSService{
		sender:        sender,
		verifySign:    verifySign,
		verifyTplID:   verifyTplID,
		verifyCodeSvc: NewVerificationCodeService(),
	}
}

// Enabled 短信服务是否可用
func (s *SMSService) Enabled() bool {
	return s != nil && s.sender != nil
}

// SendVerificationCode 发送验证码（上游受理即视为已发送，送达结果以回执为准；码值以 Redis 为权威）
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

	if err := s.sender.SendVerificationSms(phone, s.verifyTplID, s.verifySign, code); err != nil {
		return fmt.Errorf("发送短信失败: %w", err)
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