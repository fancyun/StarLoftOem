package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
)

// 上传图片访问签名：对外图片地址携带 ?e=过期时间(Unix 秒)&s=签名，任意来源凭有效签名即可读取，
// 从而不依赖调用方 IP（上游出口 IP 变动不再导致拉图失败）。未携带签名或签名已过期时，
// 回落到「登录态（管理员/上传者本人）」校验。
// 签名 = HMAC-SHA256(密钥, 相对路径 + "|" + 过期时间) 取小写 hex；密钥取 .env JWT_SECRET 并加固定前缀做域分离。
// 更换 JWT_SECRET 会使已签发的图片地址（含上游已报备材料）立即失效，需重新签发。

const uploadSignSalt = "starloft:upload-url:v1:"

var uploadSignKey []byte

// InitUploadSigner 初始化上传图片访问签名密钥（取自 JWT_SECRET，需满足 JWT 同强度）
func InitUploadSigner(secret string) error {
	if len(secret) < 32 {
		return errors.New("JWT_SECRET 长度不足 32 字节，无法用于图片访问签名")
	}
	uploadSignKey = []byte(uploadSignSalt + secret)
	return nil
}

// SignUploadPath 为上传图片相对路径（yyyyMMdd/{md5}.jpg）生成访问签名
func SignUploadPath(relPath string, expireAt int64) string {
	return uploadSignHex(relPath, expireAt)
}

// VerifyUploadSign 校验访问签名与时效，expire/sign 为 URL 查询参数 ?e= 与 ?s=
func VerifyUploadSign(relPath, expire, sign string) bool {
	if len(uploadSignKey) == 0 || relPath == "" || expire == "" || sign == "" {
		return false
	}
	expireAt, err := strconv.ParseInt(expire, 10, 64)
	if err != nil || expireAt < time.Now().Unix() {
		return false
	}
	return hmac.Equal([]byte(uploadSignHex(relPath, expireAt)), []byte(strings.ToLower(sign)))
}

// uploadSignHex 计算签名（路径与过期时间共同参与，路径被替换则签名失效）
func uploadSignHex(relPath string, expireAt int64) string {
	mac := hmac.New(sha256.New, uploadSignKey)
	mac.Write([]byte(relPath + "|" + strconv.FormatInt(expireAt, 10)))
	return hex.EncodeToString(mac.Sum(nil))
}
