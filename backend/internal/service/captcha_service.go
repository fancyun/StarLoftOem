package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

var (
	ErrCaptchaVerifyFailed = errors.New("captcha verify failed")
)

type CaptchaService struct {
	secretId     string
	secretKey    string
	captchaAppId string
	appSecretKey string
	endpoint     string
}

// NewCaptchaService 创建腾讯天御验证码服务（腾讯云API 3.0 - DescribeCaptchaResult）
func NewCaptchaService(secretId, secretKey, captchaAppId, appSecretKey string) *CaptchaService {
	return &CaptchaService{
		secretId:     secretId,
		secretKey:    secretKey,
		captchaAppId: captchaAppId,
		appSecretKey: appSecretKey,
		endpoint:     "captcha.tencentcloudapi.com",
	}
}

// VerifyCaptcha 验证腾讯天御验证码
// ticket: 前端验证成功后返回的票据
// randStr: 前端验证成功后返回的随机字符串
// userIP: 用户IP地址
func (s *CaptchaService) VerifyCaptcha(ticket, randStr, userIP string) error {
	// 未配置腾讯云凭据（本地/演示环境）时直接放行，避免阻断登录与注册流程。
	// 生产环境必须配置 TENCENT_SECRET_ID / TENCENT_SECRET_KEY / TENCENT_CAPTCHA_SECRET。
	if s.secretId == "" || s.secretKey == "" || s.appSecretKey == "" {
		return nil
	}

	timestamp := time.Now().Unix()

	// CaptchaAppId 官方要求为 Integer 类型
	captchaAppIdInt, err := strconv.ParseInt(s.captchaAppId, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid captcha app id: %w", err)
	}

	requestBody := map[string]interface{}{
		"CaptchaType":  9,
		"Ticket":       ticket,
		"UserIp":       userIP,
		"Randstr":      randStr,
		"CaptchaAppId": captchaAppIdInt,
		"AppSecretKey": s.appSecretKey,
	}

	bodyBytes, _ := json.Marshal(requestBody)
	bodyStr := string(bodyBytes)

	authorization := s.buildAuthorization(bodyStr, timestamp)

	url := "https://" + s.endpoint + "/"
	req, err := http.NewRequest("POST", url, strings.NewReader(bodyStr))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", s.endpoint)
	req.Header.Set("X-TC-Action", "DescribeCaptchaResult")
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Version", "2019-07-22")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response failed: %w", err)
	}

	var result struct {
		Response struct {
			CaptchaCode int    `json:"CaptchaCode"`
			CaptchaMsg  string `json:"CaptchaMsg"`
			EvilLevel   int    `json:"EvilLevel"`
			RequestId   string `json:"RequestId"`
			Error       *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("parse response failed: %w", err)
	}

	if result.Response.Error != nil {
		return fmt.Errorf("captcha api error: %s", result.Response.Error.Message)
	}

	// CaptchaCode 返回值说明：
	// 1  - 验证成功
	// 7  - captcha no match（Randstr 无效）
	// 8  - ticket 过期（有效期 5 分钟）
	// 9  - ticket 重复使用
	// 15 - decrypt fail（Ticket 无效）
	// 16 - appid-ticket mismatch（CaptchaAppId 错误）
	// 21 - diff 异常（容灾票据或风控拦截）
	// 100 - appid-secretkey-ticket mismatch（参数验证错误）
	if result.Response.CaptchaCode != 1 {
		return fmt.Errorf("captcha verify failed (code %d): %s", result.Response.CaptchaCode, result.Response.CaptchaMsg)
	}

	return nil
}

// buildAuthorization 构建腾讯云API 3.0 TC3-HMAC-SHA256签名
func (s *CaptchaService) buildAuthorization(bodyStr string, timestamp int64) string {
	// 1. 拼接规范请求串
	signedHeaders := "content-type;host"
	canonicalHeaders := "content-type:application/json\nhost:" + s.endpoint + "\n"
	hashedRequestPayload := sha256hex(bodyStr)
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + hashedRequestPayload

	// 2. 拼接待签名字符串
	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := date + "/captcha/tc3_request"
	hashedCanonicalRequest := sha256hex(canonicalRequest)
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" + credentialScope + "\n" + hashedCanonicalRequest

	// 3. 计算签名
	secretDate := hmacSHA256([]byte("TC3"+s.secretKey), date)
	secretService := hmacSHA256(secretDate, "captcha")
	secretSigning := hmacSHA256(secretService, "tc3_request")
	signature := hex.EncodeToString(hmacSHA256(secretSigning, stringToSign))

	// 4. 拼接Authorization
	return "TC3-HMAC-SHA256 " +
		"Credential=" + s.secretId + "/" + credentialScope + ", " +
		"SignedHeaders=" + signedHeaders + ", " +
		"Signature=" + signature
}

func sha256hex(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return hex.EncodeToString(h.Sum(nil))
}

func hmacSHA256(key []byte, str string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(str))
	return h.Sum(nil)
}
