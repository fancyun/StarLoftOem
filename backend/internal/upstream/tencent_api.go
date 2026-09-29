package upstream

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// TencentClient 腾讯云 API 3.0 客户端基座：
// 封装 TC3-HMAC-SHA256 签名与 HTTP 请求，各腾讯云产品客户端复用。
type TencentClient struct {
	secretID  string
	secretKey string
	region    string
}

// NewTencentClient 创建腾讯云 API 3.0 客户端（region 如 ap-guangzhou）。
func NewTencentClient(secretID, secretKey, region string) *TencentClient {
	return &TencentClient{secretID: secretID, secretKey: secretKey, region: region}
}

// Call 调用指定腾讯云产品动作，返回响应体。body 为请求参数字典。
func (c *TencentClient) Call(service, action, version string, body map[string]interface{}) ([]byte, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request failed: %w", err)
	}
	bodyStr := string(bodyBytes)
	timestamp := time.Now().Unix()

	// host 必须使用不带 scheme/host 头的官方域名，签名与请求一致
	host := service + ".tencentcloudapi.com"
	authorization := tencentAuthorization(c.secretID, c.secretKey, service, c.region, bodyStr, timestamp)

	req, err := http.NewRequest("POST", "https://"+host+"/", strings.NewReader(bodyStr))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Host", host)
	req.Header.Set("X-TC-Action", action)
	req.Header.Set("X-TC-Timestamp", strconv.FormatInt(timestamp, 10))
	req.Header.Set("X-TC-Version", version)
	req.Header.Set("X-TC-Region", c.region)

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}
	return out, nil
}

// tencentAuthorization 计算腾讯云 API 3.0 TC3-HMAC-SHA256 签名头。
func tencentAuthorization(secretID, secretKey, service, region, bodyStr string, timestamp int64) string {
	signedHeaders := "content-type;host"
	canonicalHeaders := "content-type:application/json\nhost:" + service + ".tencentcloudapi.com\n"
	hashedPayload := tc3SHA256Hex(bodyStr)
	canonicalRequest := "POST\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + hashedPayload

	date := time.Unix(timestamp, 0).UTC().Format("2006-01-02")
	credentialScope := date + "/" + service + "/tc3_request"
	hashedCanonicalRequest := tc3SHA256Hex(canonicalRequest)
	stringToSign := "TC3-HMAC-SHA256\n" + strconv.FormatInt(timestamp, 10) + "\n" + credentialScope + "\n" + hashedCanonicalRequest

	secretDate := tc3HMACBytes([]byte("TC3"+secretKey), date)
	secretService := tc3HMACBytes(secretDate, service)
	secretSigning := tc3HMACBytes(secretService, "tc3_request")
	signature := hex.EncodeToString(tc3HMACBytes(secretSigning, stringToSign))

	return "TC3-HMAC-SHA256 " +
		"Credential=" + secretID + "/" + credentialScope + ", " +
		"SignedHeaders=" + signedHeaders + ", " +
		"Signature=" + signature
}

func tc3SHA256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func tc3HMACBytes(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

// TencentError 从腾讯云响应体中提取业务错误（Error.Code / Error.Message）。
func TencentError(respBody []byte) error {
	var r struct {
		Response struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return fmt.Errorf("parse response failed: %w", err)
	}
	if r.Response.Error != nil && r.Response.Error.Code != "" {
		return fmt.Errorf("tencent %s: %s", r.Response.Error.Code, r.Response.Error.Message)
	}
	return nil
}
