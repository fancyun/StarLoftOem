package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type JWTClaims struct {
	UserID   int64  `json:"user_id"`
	Phone    string `json:"phone"`
	UserType string `json:"user_type"` // "user" or "admin"
	jwt.RegisteredClaims
}

type JWTManager struct {
	secretKey string
}

func NewJWTManager(secretKey string) *JWTManager {
	return &JWTManager{secretKey: secretKey}
}

// GenerateToken 生成 JWT Token
func (m *JWTManager) GenerateToken(userID int64, phone, userType string, expireDuration time.Duration) (string, error) {
	claims := JWTClaims{
		UserID:   userID,
		Phone:    phone,
		UserType: userType,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(m.secretKey))
}

// ValidateToken 验证 JWT Token
func (m *JWTManager) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(m.secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// SignatureManager API 签名管理器
type SignatureManager struct{}

func NewSignatureManager() *SignatureManager {
	return &SignatureManager{}
}

// GenerateHMACSHA256 生成 HMAC-SHA256 签名
func (m *SignatureManager) GenerateHMACSHA256(secret, data string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMACSHA256 验证 HMAC-SHA256 签名
func (m *SignatureManager) VerifyHMACSHA256(secret, data, signature string) bool {
	expectedSignature := m.GenerateHMACSHA256(secret, data)
	return hmac.Equal([]byte(expectedSignature), []byte(signature))
}

// VerifyTimestamp 验证时间戳（防重放攻击）
func (m *SignatureManager) VerifyTimestamp(timestamp int64, allowedDiffSeconds int64) bool {
	now := time.Now().Unix()
	diff := now - timestamp
	if diff < 0 {
		diff = -diff
	}
	return diff <= allowedDiffSeconds
}
