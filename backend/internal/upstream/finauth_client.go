package upstream

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oemrpa/internal/logstore"
)

var (
	ErrFinAuthAPIFailed = errors.New("finauth api failed")
	// ErrFinAuthDataDestroyed 上游认证数据已销毁（get_result 返回 403 DATA_DESTROYED）：
	// 该笔核身不可能再出结果，属终态，调用方应终结订单并退还已扣费用，
	// 否则订单会一直停在「认证中」，下游/控制台无限轮询且已扣费用挂着不退。
	ErrFinAuthDataDestroyed = errors.New("finauth data destroyed")
)

// ErrorCodeDataDestroyed 上游「认证数据已销毁」错误码（error_message 取值）
const ErrorCodeDataDestroyed = "DATA_DESTROYED"

// isDataDestroyed 判断上游错误信息是否为「认证数据已销毁」
func isDataDestroyed(errorMessage string) bool {
	return strings.EqualFold(strings.TrimSpace(errorMessage), ErrorCodeDataDestroyed)
}

// FinAuthClient FinAuth H5 Plus 真实 API 客户端
// 使用 HMAC 签名认证
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_will_access_guidence_plus
type FinAuthClient struct {
	apiKey    string
	apiSecret string
	baseURL   string
	signer    *FinAuthSigner
	client    *http.Client
}

// NewFinAuthClient 创建 FinAuth 客户端
func NewFinAuthClient(baseURL, apiKey, apiSecret string) *FinAuthClient {
	return &FinAuthClient{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   baseURL,
		signer:    NewFinAuthSigner(apiSecret),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetToken 获取认证 token
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_plus_get_token
func (c *FinAuthClient) GetToken(req *GetTokenRequest) (*GetTokenResponse, error) {
	// 生成签名（api_key 编码在签名中，不需要单独传递）
	sign, err := c.signer.GenerateSign(c.apiKey, req.SignVersion)
	if err != nil {
		return nil, fmt.Errorf("generate sign failed: %w", err)
	}

	// 构建 multipart/form-data 请求体
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// 必选参数
	writer.WriteField("sign", sign)
	writer.WriteField("sign_version", req.SignVersion)
	writer.WriteField("return_url", req.ReturnURL)
	writer.WriteField("notify_url", req.NotifyURL)
	writer.WriteField("biz_no", req.BizNo)
	writer.WriteField("scene_id", req.SceneID)
	writer.WriteField("comparison_type", req.ComparisonType)
	writer.WriteField("uuid", req.UUID)

	// 身份证拍摄模式参数
	if req.IDCardMode != "" {
		writer.WriteField("idcard_mode", req.IDCardMode)
	}
	if req.IDCardName != "" {
		writer.WriteField("idcard_name", req.IDCardName)
	}
	if req.IDCardNumber != "" {
		writer.WriteField("idcard_number", req.IDCardNumber)
	}

	// 可选参数
	if req.BizExtraData != "" {
		writer.WriteField("biz_extra_data", req.BizExtraData)
	}
	if req.EncryptionType != "" {
		writer.WriteField("encryption_type", req.EncryptionType)
	}

	writer.Close()

	// 发送 HTTP 请求
	url := fmt.Sprintf("%s/finauth/lite/plus/get_token", c.baseURL)
	log.Printf("FinAuth GetToken 请求: URL=%s, BizNo=%s", url, req.BizNo)

	httpReq, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())

	// 执行请求
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}
	log.Printf("FinAuth GetToken 响应: Status=%d, Body=%s", httpResp.StatusCode, redactBody(respBody))
	// 记录对实名认证接口的调用日志（sys_call_log，direction=request）
	ref := ""
	if httpResp.StatusCode == 200 {
		ref = "http_ok"
	}
	logstore.RecordSysCall("request", "finauth", req.BizNo, 0, ref, "", redactBody(respBody), httpOkStatus(httpResp.StatusCode))

	// 解析响应
	var resp GetTokenResponse
	err = json.Unmarshal(respBody, &resp)
	if err != nil {
		return nil, fmt.Errorf("unmarshal response failed: %w", err)
	}

	// 检查错误
	if resp.ErrorMessage != "" {
		log.Printf("FinAuth GetToken API 错误: %s", resp.ErrorMessage)
		return nil, fmt.Errorf("%w: %s", ErrFinAuthAPIFailed, resp.ErrorMessage)
	}

	// 检查 token 是否为空
	if resp.Token == "" {
		return nil, fmt.Errorf("%w: empty token in response", ErrFinAuthAPIFailed)
	}

	return &resp, nil
}

// GetResult 查询认证结果（GET 请求）
// 文档: https://www.yljz.com/document/finauth-guide-docs/h5_will_plus_get_result
func (c *FinAuthClient) GetResult(req *GetResultRequest) (*GetResultResponse, error) {
	// 生成签名（api_key 编码在签名中，不需要单独传递）
	sign, err := c.signer.GenerateSign(c.apiKey, req.SignVersion)
	if err != nil {
		return nil, fmt.Errorf("generate sign failed: %w", err)
	}

	// 构建 GET 请求 URL（带签名参数）
	apiURL := fmt.Sprintf("%s/finauth/lite/plus/get_result", c.baseURL)
	queryParams := url.Values{}
	queryParams.Set("sign", sign)
	queryParams.Set("sign_version", req.SignVersion)
	queryParams.Set("biz_id", req.BizID)

	fullURL := fmt.Sprintf("%s?%s", apiURL, queryParams.Encode())
	// 不打印完整 URL（query 中携带 sign 签名，防泄露/重放），仅打印接口地址与业务 ID
	log.Printf("FinAuth GetResult 请求: URL=%s, BizID=%s", apiURL, req.BizID)

	httpReq, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	// 执行请求
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer httpResp.Body.Close()

	// 读取响应
	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}
	log.Printf("FinAuth GetResult 响应: Status=%d, Body=%s", httpResp.StatusCode, redactBody(respBody))
	// 记录对实名认证接口的调用日志（sys_call_log，direction=request）
	logstore.RecordSysCall("request", "finauth", "", 0, req.BizID, "", redactBody(respBody), httpOkStatus(httpResp.StatusCode))

	// 解析响应（错误响应体的字段名可能与预期不同，解析失败不影响下面按状态码与响应体归类错误）
	var resp GetResultResponse
	_ = json.Unmarshal(respBody, &resp)

	// HTTP 层非 2xx：一律按失败处理。上游「超过可查询时间或超过最多可查询次数」返回 403 DATA_DESTROYED，
	// 该错误码可能出现在 error_message / error_code / message 等字段，故对响应体做宽松匹配，
	// 避免字段名不一致时把「结果已不可取回」误判成「认证中」而让订单一直挂着。
	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		errMsg := strings.TrimSpace(resp.ErrorMessage)
		if errMsg == "" {
			errMsg = fmt.Sprintf("HTTP %d", httpResp.StatusCode)
		}
		if isDataDestroyed(errMsg) || bytes.Contains(respBody, []byte(ErrorCodeDataDestroyed)) {
			return nil, fmt.Errorf("%w: %s", ErrFinAuthDataDestroyed, errMsg)
		}
		return nil, fmt.Errorf("%w: %s", ErrFinAuthAPIFailed, errMsg)
	}

	// 检查错误（2xx 但携带 error_message）
	if resp.ErrorMessage != "" {
		log.Printf("FinAuth GetResult API 错误: %s", resp.ErrorMessage)
		// 结果已不可取回：终态语义，单独归类以便调用方终结订单并退款
		if isDataDestroyed(resp.ErrorMessage) {
			return nil, fmt.Errorf("%w: %s", ErrFinAuthDataDestroyed, resp.ErrorMessage)
		}
		return nil, fmt.Errorf("%w: %s", ErrFinAuthAPIFailed, resp.ErrorMessage)
	}

	return &resp, nil
}

// VerifySign 验证回调签名
func (c *FinAuthClient) VerifySign(jsonData, receivedSign string) bool {
	return c.signer.VerifyNotifySign(jsonData, receivedSign, SignVersionHMACSHA256)
}

// DownloadVideo 下载上游返回的验证视频原始字节（AES-ECB 加密，需经 DecryptVideoAESECB 解密后存储）
// video URL 有效期约 1 天，认证结果确定后应立即下载保存
func (c *FinAuthClient) DownloadVideo(videoURL string) ([]byte, error) {
	if videoURL == "" {
		return nil, fmt.Errorf("empty video url")
	}
	httpReq, err := http.NewRequest("GET", videoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create video request failed: %w", err)
	}
	httpResp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("download video failed: %w", err)
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download video failed: status=%d", httpResp.StatusCode)
	}
	data, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("read video failed: %w", err)
	}
	if len(data) == 0 {
		return nil, fmt.Errorf("video data empty")
	}
	return data, nil
}

// sensitiveKeys 日志脱敏：命中该集合的字段一律替换为 ***
var sensitiveKeys = map[string]bool{
	"idcard_name":      true,
	"idcard_number":    true,
	"id_card":          true,
	"name":             true,
	"images":           true,
	"liveness_result":  true,
	"verify_result":    true,
	"will_result":      true,
	"verify_risk_info": true,
	"device_risk_info": true,
	// 核验敏感字段（姓名/证件/手机号/银行卡/企业要素）
	"idcard":      true,
	"mobile":      true,
	"bankcard":    true,
	"companyname": true,
	"creditno":    true,
	"legalperson": true,
	"legalname":   true,
	"legalidcard": true,
	"companyName": true,
	"creditNo":    true,
	"legalName":   true,
	"legalIdcard": true,
}

// httpOkStatus 将 HTTP 状态码转换为 sys_call_log.status（200 为 1 成功，其余 0 失败）
func httpOkStatus(code int) int {
	if code == 200 {
		return 1
	}
	return 0
}

// RedactPayload 对第三方请求/响应体做脱敏（敏感字段置 ***）并截断到 1000 字符，
// 供回调/调用日志记录使用，避免人脸图片、证件信息等隐私数据落日志。
func RedactPayload(body []byte) string { return redactBody(body) }

// redactBody 对上游响应 JSON 做脱敏后再输出日志，防止人脸图片、证件信息等隐私数据落入日志
func redactBody(body []byte) string {
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		s := string(body)
		if len(s) > 500 {
			return s[:500] + "...(truncated)"
		}
		return s
	}

	out, err := json.Marshal(redactJSON(data))
	if err != nil {
		return "(redact failed)"
	}
	s := string(out)
	if len(s) > 1000 {
		return s[:1000] + "...(truncated)"
	}
	return s
}

func redactJSON(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		for k := range v {
			if sensitiveKeys[k] {
				v[k] = "***"
			} else {
				v[k] = redactJSON(v[k])
			}
		}
		return v
	case []interface{}:
		for i := range v {
			v[i] = redactJSON(v[i])
		}
		return v
	default:
		return v
	}
}
