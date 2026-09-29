package upstream

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"time"
)

// SignVersion 签名算法版本
const (
	SignVersionHMACSHA1   = "hmac_sha1"
	SignVersionHMACSHA256 = "hmac_sha256"
)

// FinAuthSigner FinAuth HMAC 签名器
type FinAuthSigner struct {
	apiSecret string
}

// NewFinAuthSigner 创建签名器
func NewFinAuthSigner(apiSecret string) *FinAuthSigner {
	return &FinAuthSigner{apiSecret: apiSecret}
}

// GenerateSign 生成请求签名
// 鉴权文档: https://www.yljz.com/document/finauth-guide-docs/adv_authentication
// 签名步骤:
//  1. 构建 raw = "a={api_key}&b={expire_time}&c={current_time}&d={random}"
//  2. 计算 HMAC(api_secret, raw) 得到二进制摘要
//  3. sign = Base64(HMAC摘要 + raw)
func (s *FinAuthSigner) GenerateSign(apiKey, signVersion string) (string, error) {
	currentTime := time.Now().Unix()
	expireTime := currentTime + 100  // 签名有效期 100 秒
	random := rand.Intn(10000000000) // 10 位随机数

	raw := fmt.Sprintf("a=%s&b=%d&c=%d&d=%d", apiKey, expireTime, currentTime, random)

	var hmacDigest []byte
	switch signVersion {
	case SignVersionHMACSHA1:
		hmacDigest = hmacSHA1Bytes(s.apiSecret, raw)
	case SignVersionHMACSHA256:
		hmacDigest = hmacSHA256Bytes(s.apiSecret, raw)
	default:
		return "", fmt.Errorf("unsupported sign_version: %s", signVersion)
	}

	// 拼接 HMAC 摘要 + raw 字符串，然后 Base64 编码
	signContent := append(hmacDigest, []byte(raw)...)
	sign := base64.StdEncoding.EncodeToString(signContent)

	return sign, nil
}

// VerifyNotifySign 验证回调签名
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_plus_return_notify_url
// 回调签名规则: sign = sha1(API_SECRET + json_data) 或 sign = sha256(API_SECRET + json_data)
// 注意：这是 SHA1/SHA256(secret+data)，不是 HMAC！
func (s *FinAuthSigner) VerifyNotifySign(jsonData, receivedSign, signVersion string) bool {
	var expectedHex string
	switch signVersion {
	case SignVersionHMACSHA1:
		expectedHex = sha1Hex(s.apiSecret + jsonData)
	case SignVersionHMACSHA256:
		expectedHex = sha256Hex(s.apiSecret + jsonData)
	default:
		// 兼容：先尝试 SHA256，再尝试 SHA1
		expectedHex = sha256Hex(s.apiSecret + jsonData)
		if hmac.Equal([]byte(expectedHex), []byte(receivedSign)) {
			return true
		}
		expectedHex = sha1Hex(s.apiSecret + jsonData)
	}
	return hmac.Equal([]byte(expectedHex), []byte(receivedSign))
}

func sha1Hex(data string) string {
	h := sha1.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func sha256Hex(data string) string {
	h := sha256.New()
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA1Bytes(secret, data string) []byte {
	h := hmac.New(sha1.New, []byte(secret))
	h.Write([]byte(data))
	return h.Sum(nil)
}

func hmacSHA256Bytes(secret, data string) []byte {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return h.Sum(nil)
}
