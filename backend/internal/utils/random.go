package utils

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
)

// GenerateRandomKey 生成指定字节长度的随机十六进制密钥（使用密码学安全随机源）
// 返回长度 = length * 2 的十六进制字符串
func GenerateRandomKey(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// GenerateRandomDigits 生成指定长度的随机数字字符串（使用密码学安全随机源）
// 每位数字取值范围 0-9，来源为密码学随机字节取模
func GenerateRandomDigits(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	var sb strings.Builder
	sb.Grow(length)
	for _, by := range b {
		sb.WriteByte('0' + by%10)
	}
	return sb.String()
}
