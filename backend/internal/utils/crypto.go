package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"strings"
)

// 数据加密：AES-256-GCM，密钥来自 .env DATA_ENCRYPT_KEY（必须为 32 字节）。
// 密文统一带 "enc:v1:" 前缀，便于识别与幂等迁移；无前缀视为历史明文原样返回。
// 用途：存储层敏感字段加密（实名证件号、API Secret 等），内存中保持明文供业务使用。

const encPrefix = "enc:v1:"

var gcm cipher.AEAD

// InitCrypto 初始化全局 AES-GCM 加密器（key 需为 32 字节）
func InitCrypto(key string) error {
	if len(key) != 32 {
		return errors.New("DATA_ENCRYPT_KEY 必须为 32 字节")
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return err
	}
	gcm, err = cipher.NewGCM(block)
	if err != nil {
		return err
	}
	return nil
}

// EncryptText 加密字符串；空串原样返回；已是 enc:v1: 前缀则幂等跳过
func EncryptText(plain string) string {
	if plain == "" || strings.HasPrefix(plain, encPrefix) {
		return plain
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return plain
	}
	ct := gcm.Seal(nil, nonce, []byte(plain), nil)
	return encPrefix + base64.StdEncoding.EncodeToString(append(nonce, ct...))
}

// DecryptText 解密 enc:v1: 密文；无前缀视为明文原样返回（兼容存量数据）
func DecryptText(s string) string {
	if s == "" || !strings.HasPrefix(s, encPrefix) {
		return s
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(s, encPrefix))
	if err != nil || len(raw) < gcm.NonceSize() {
		return s
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return s
	}
	return string(plain)
}
