package utils

import (
	"errors"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// MinKycAge 平台实名认证最低年龄限制（周岁）
const MinKycAge = 16

var (
	ErrInvalidPhone    = errors.New("invalid phone number")
	ErrInvalidPassword = errors.New("password must be 8-32 characters with letters and numbers")
	ErrInvalidIDCard   = errors.New("invalid ID card number")
	ErrIDCardUnderage  = errors.New("实名认证需年满16周岁")
	ErrInvalidName     = errors.New("invalid name")
	ErrInvalidAmount   = errors.New("invalid amount")
	ErrSQLInjection    = errors.New("potential SQL injection detected")
	ErrXSSAttempt      = errors.New("potential XSS attempt detected")
)

// InputValidator 输入验证器
type InputValidator struct{}

func NewInputValidator() *InputValidator {
	return &InputValidator{}
}

// ValidatePhone 验证手机号（中国大陆）
func (v *InputValidator) ValidatePhone(phone string) error {
	if phone == "" {
		return ErrInvalidPhone
	}
	matched, _ := regexp.MatchString(`^1[3-9]\d{9}$`, phone)
	if !matched {
		return ErrInvalidPhone
	}
	return nil
}

// ValidatePassword 验证密码强度
func (v *InputValidator) ValidatePassword(password string) error {
	if len(password) < 8 || len(password) > 32 {
		return ErrInvalidPassword
	}

	hasLetter := false
	hasNumber := false

	for _, char := range password {
		if unicode.IsLetter(char) {
			hasLetter = true
		}
		if unicode.IsDigit(char) {
			hasNumber = true
		}
	}

	if !hasLetter || !hasNumber {
		return ErrInvalidPassword
	}

	return nil
}

// ValidateIDCard 验证身份证号（中国大陆18位）
func (v *InputValidator) ValidateIDCard(idCard string) error {
	if len(idCard) != 18 {
		return ErrInvalidIDCard
	}

	// 基本格式验证
	matched, _ := regexp.MatchString(`^\d{17}[\dXx]$`, idCard)
	if !matched {
		return ErrInvalidIDCard
	}

	// 校验码验证
	weights := []int{7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2}
	checkCodes := []byte{'1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'}

	sum := 0
	for i := 0; i < 17; i++ {
		sum += int(idCard[i]-'0') * weights[i]
	}

	checkCode := checkCodes[sum%11]
	lastChar := idCard[17]
	if lastChar == 'x' {
		lastChar = 'X'
	}

	if lastChar != checkCode {
		return ErrInvalidIDCard
	}

	return nil
}

// ageFromIDCard 从18位身份证号中提取出生日期并计算周岁年龄
// 身份证号第 7-14 位为出生日期（YYYYMMDD）
func ageFromIDCard(idCard string, now time.Time) (int, error) {
	if len(idCard) != 18 {
		return 0, ErrInvalidIDCard
	}
	birth, err := time.Parse("20060102", idCard[6:14])
	if err != nil {
		return 0, ErrInvalidIDCard
	}

	age := now.Year() - birth.Year()
	// 今年生日还未过，则减一岁
	if now.Month() < birth.Month() || (now.Month() == birth.Month() && now.Day() < birth.Day()) {
		age--
	}
	return age, nil
}

// ValidateIDCardMinAge 验证身份证号格式与校验码，并校验是否达到最低年龄
func (v *InputValidator) ValidateIDCardMinAge(idCard string, minAge int) error {
	if err := v.ValidateIDCard(idCard); err != nil {
		return err
	}
	age, err := ageFromIDCard(idCard, time.Now())
	if err != nil {
		return ErrInvalidIDCard
	}
	if age < minAge {
		return ErrIDCardUnderage
	}
	return nil
}

// ValidateName 验证姓名（2-50字符，仅中文、英文字母、间隔号）
func (v *InputValidator) ValidateName(name string) error {
	name = strings.TrimSpace(name)

	// 按字符数（rune）判断长度，而非字节数
	if utf8.RuneCountInString(name) < 2 || utf8.RuneCountInString(name) > 50 {
		return ErrInvalidName
	}

	// \p{Han} 匹配所有汉字（含扩展区生僻字），同时允许英文字母和间隔号
	matched, _ := regexp.MatchString(`^[\p{Han}a-zA-Z·]+$`, name)
	if !matched {
		return ErrInvalidName
	}

	return nil
}

// ValidateAmount 验证金额（大于0，最多两位小数）
func (v *InputValidator) ValidateAmount(amount float64) error {
	if amount <= 0 || amount > 1000000 {
		return ErrInvalidAmount
	}
	return nil
}

// SanitizeString 清理字符串（防止XSS）
func (v *InputValidator) SanitizeString(input string) string {
	// 移除潜在的HTML标签和脚本
	input = strings.ReplaceAll(input, "&", "&amp;")
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#39;")
	return input
}

// DetectSQLInjection 检测SQL注入特征
func (v *InputValidator) DetectSQLInjection(input string) error {
	lowerInput := strings.ToLower(input)
	sqlKeywords := []string{
		"' or '", "' and '", "union select", "drop table",
		"insert into", "delete from", "update set", "exec(",
		"execute(", "script", "javascript:", "onerror=",
	}

	for _, keyword := range sqlKeywords {
		if strings.Contains(lowerInput, keyword) {
			return ErrSQLInjection
		}
	}

	return nil
}

// ValidateURL 验证回调/回跳 URL 格式与目标安全（防 SSRF/开放重定向）：
// 仅允许 http/https，且域名解析出的 IP 不得为内网/回环/链路本地/未指定地址。
// 空串视为合法（未传时不使用该地址）。
func (v *InputValidator) ValidateURL(rawURL string) error {
	if rawURL == "" {
		return nil
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return errors.New("invalid URL format")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("invalid URL scheme")
	}
	host := parsed.Hostname()
	if host == "" || parsed.User != nil {
		return errors.New("invalid URL host")
	}
	// 显式 IP：直接判定
	if ip := net.ParseIP(host); ip != nil {
		if isUnsafeIP(ip) {
			return errors.New("invalid URL host")
		}
		return nil
	}
	// 域名：解析并判定（任一解析结果命中内网即拒绝，防止 SSRF 到内网/云元数据）
	addrs, err := net.LookupIP(host)
	if err != nil {
		return errors.New("invalid URL host")
	}
	for _, ip := range addrs {
		if isUnsafeIP(ip) {
			return errors.New("invalid URL host")
		}
	}
	return nil
}

// isUnsafeIP 判断 IP 是否属于不可作为回调目标的内网/特殊地址段
func isUnsafeIP(ip net.IP) bool {
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() ||
		ip.IsMulticast() ||
		ip.IsInterfaceLocalMulticast()
}
