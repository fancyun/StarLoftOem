package service

import (
	"fmt"
	"math/rand"
	"time"

	"oemrpa/internal/redis"
)

const (
	// 短信验证码前缀
	SMSCodePrefix = "sms:code:"
	// 短信验证码有效期（5分钟）
	SMSCodeExpire = 5 * time.Minute
	// 短信发送频率限制前缀
	SMSRateLimitPrefix = "sms:rate:"
	// 短信发送频率限制时间（60秒）
	SMSRateLimitExpire = 60 * time.Second
)

// VerificationCodeService 验证码服务
type VerificationCodeService struct{}

// NewVerificationCodeService 创建验证码服务实例
func NewVerificationCodeService() *VerificationCodeService {
	return &VerificationCodeService{}
}

// GenerateCode 生成6位数字验证码
func (s *VerificationCodeService) GenerateCode() string {
	code := rand.Intn(900000) + 100000 // 100000-999999
	return fmt.Sprintf("%06d", code)
}

// SaveCode 保存验证码到 Redis
func (s *VerificationCodeService) SaveCode(phone, code string) error {
	key := SMSCodePrefix + phone
	return redis.Set(key, code, SMSCodeExpire)
}

// VerifyCode 验证验证码
func (s *VerificationCodeService) VerifyCode(phone, code string) (bool, error) {
	key := SMSCodePrefix + phone
	storedCode, err := redis.Get(key)
	if err != nil {
		// 键不存在或已过期
		return false, nil
	}

	if storedCode == code {
		// 验证成功后删除验证码（防止重复使用）
		_ = redis.Del(key)
		return true, nil
	}

	return false, nil
}

// CheckSendRateLimit 检查短信发送频率限制
// 返回：(是否可以发送, 剩余秒数, error)
func (s *VerificationCodeService) CheckSendRateLimit(phone string) (bool, int, error) {
	key := SMSRateLimitPrefix + phone

	// 检查是否存在限制
	exists, err := redis.Exists(key)
	if err != nil {
		return false, 0, err
	}

	if exists > 0 {
		// 获取剩余过期时间
		ttl, err := redis.TTL(key)
		if err != nil {
			return false, 0, err
		}
		return false, int(ttl.Seconds()), nil
	}

	return true, 0, nil
}

// SetSendRateLimit 设置短信发送频率限制
func (s *VerificationCodeService) SetSendRateLimit(phone string) error {
	key := SMSRateLimitPrefix + phone
	return redis.Set(key, "1", SMSRateLimitExpire)
}
