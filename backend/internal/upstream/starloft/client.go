// Package starloft 封装 StarLoft 平台开放 API（/v1/*）客户端。
// OEM 系统不再直连真实上游供应商，短信、人脸核验、系统验证码短信与账户实名扫脸
// 一律以 StarLoft 平台为唯一上游，经本包调用其开放 API 完成。
package starloft

import (
	"bytes"
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

	"oemrpa/internal/logstore"
	"oemrpa/internal/site"
)

// APIError StarLoft 平台返回的业务错误（响应 code != 0 或 HTTP 状态非 2xx）。
type APIError struct {
	Code    int    // 平台业务码
	Message string // 平台返回信息
	Status  int    // HTTP 状态码
}

func (e *APIError) Error() string {
	return fmt.Sprintf("StarLoft 平台返回错误: code=%d http=%d message=%s", e.Code, e.Status, e.Message)
}

// Client StarLoft 平台开放 API 客户端（签名：X-Sign = HMAC-SHA256(APISecret, 原始请求体) 的十六进制小写）
type Client struct {
	baseURL          string // API 基址（如 https://api.example.com，路径为 /v1/*）
	apiKey           string
	apiSecret        string
	marketingEnabled bool // 上游账号是否已开通营销短信通道（由后台配置）
	http             *http.Client
}

// New 创建 StarLoft 平台客户端；baseURL 为 API 基址（可与 /v1 路径拼接），
// apiKey/apiSecret 为平台签发的密钥对，marketingEnabled 表示上游账号是否已开通营销短信通道。
func New(baseURL, apiKey, apiSecret string, marketingEnabled bool) *Client {
	return &Client{
		baseURL:          strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:           strings.TrimSpace(apiKey),
		apiSecret:        apiSecret,
		marketingEnabled: marketingEnabled,
		http:             &http.Client{Timeout: 30 * time.Second},
	}
}

// Available 客户端是否可用（基址与密钥对齐备）
func (c *Client) Available() bool {
	return c != nil && c.baseURL != "" && c.apiKey != "" && c.apiSecret != ""
}

// envelope 平台统一响应信封 {code, message, data}
type envelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// call 调用平台接口：payload 序列化后参与签名，成功时把 data 解入 out（out 为 nil 时不解析）。
// tag 为 syscall.log 的通道标识后缀；bizNo 为可 join 的业务号（无则空）。
func (c *Client) call(method, path, tag, bizNo string, payload, out interface{}) error {
	if !c.Available() {
		return fmt.Errorf("StarLoft 平台通道未配置")
	}

	var body []byte
	if payload != nil {
		var err error
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("序列化 StarLoft 请求失败: %w", err)
		}
	}

	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write(body)
	sign := hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("构建 StarLoft 请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json;charset=utf-8")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("X-Sign", sign)
	req.Header.Set("X-Sign-Version", "hmac_sha256")
	req.Header.Set("X-Timestamp", timestamp)

	resp, err := c.http.Do(req)
	if err != nil {
		logstore.RecordSysCall("request", "starloft-"+tag, bizNo, 0, "", "", err.Error(), 0)
		return fmt.Errorf("调用 StarLoft 平台失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		logstore.RecordSysCall("request", "starloft-"+tag, bizNo, 0, "", "", err.Error(), 0)
		return fmt.Errorf("读取 StarLoft 响应失败: %w", err)
	}

	var env envelope
	if err := json.Unmarshal(raw, &env); err != nil {
		logstore.RecordSysCall("request", "starloft-"+tag, bizNo, 0, "", "", string(raw), 0)
		return fmt.Errorf("解析 StarLoft 响应失败 (http %d): %s", resp.StatusCode, string(raw))
	}

	status := 1
	if env.Code != 0 || resp.StatusCode >= 400 {
		status = 0
	}
	// 请求体含手机号/证件号等隐私字段，不落日志；仅记录响应
	logstore.RecordSysCall("request", "starloft-"+tag, bizNo, 0, "", "", string(raw), status)

	if env.Code != 0 {
		return &APIError{Code: env.Code, Message: env.Message, Status: resp.StatusCode}
	}
	if resp.StatusCode >= 400 {
		return &APIError{Code: resp.StatusCode, Message: env.Message, Status: resp.StatusCode}
	}

	if out != nil && len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("解析 StarLoft 数据失败: %w", err)
		}
	}
	return nil
}

// smsNotifyURL 短信回执/回复的下游推送地址（本平台 API 域回调路由）
func (c *Client) smsNotifyURL() string {
	return site.Platform().APIBase() + "/v1/callback/starloft/sms-report"
}