package upstream

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"oemrpa/internal/site"
)

const (
	wechatPayBaseURL = "https://api.mch.weixin.qq.com" // 微信支付 API v3 网关
)

// wechatPayNotifyURL 异步通知地址（指向平台 API 域回调路由）
func wechatPayNotifyURL() string {
	return site.Platform().APIBase() + "/v1/callback/wechat"
}

// WechatPayClient 微信支付 API v3 客户端（Native 扫码 + H5 网页支付）。
// 请求签名使用商户 API 私钥（WECHATPAY2-SHA256-RSA2048）；回调验签使用微信支付公钥；
// 回调通知体使用 APIv3 密钥做 AES-256-GCM 解密。
type WechatPayClient struct {
	appID        string
	mchID        string
	apiV3Key     []byte // APIv3 密钥（32 字节，用于回调解密）
	merchantPriv *rsa.PrivateKey
	mchSerialNo  string
	publicKey    *rsa.PublicKey // 微信支付公钥（验签用）
	httpClient   *http.Client
}

// NewWechatPayClient 创建微信支付客户端；未配置 AppID 时返回 (nil, nil)。
func NewWechatPayClient(appID, mchID, apiV3Key, merchantPrivKeyPEM, mchSerialNo, publicKeyPEM string) (*WechatPayClient, error) {
	if appID == "" {
		return nil, nil
	}
	if len(apiV3Key) != 32 {
		return nil, errors.New("APIv3 密钥必须为 32 字节")
	}
	priv, err := parseRSAPrivateKey(merchantPrivKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("解析商户 API 私钥失败: %w", err)
	}
	pub, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("解析微信支付公钥失败: %w", err)
	}
	return &WechatPayClient{
		appID:        appID,
		mchID:        mchID,
		apiV3Key:     []byte(apiV3Key),
		merchantPriv: priv,
		mchSerialNo:  mchSerialNo,
		publicKey:    pub,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// sign 构造 API v3 请求头 Authorization（WECHATPAY2-SHA256-RSA2048）。
// 签名串（5 行）：method\nURL(含查询串)\ntimestamp\nonce\nbody\n，行尾均带 \n。
func (c *WechatPayClient) sign(method, urlPath, queryString, body string) (string, error) {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	nonce := randomHex(16)
	fullURL := urlPath
	if queryString != "" {
		fullURL += "?" + queryString
	}
	message := fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n", method, fullURL, ts, nonce, body)
	hash := sha256.Sum256([]byte(message))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.merchantPriv, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`WECHATPAY2-SHA256-RSA2048 mchid="%s",nonce_str="%s",signature="%s",timestamp="%s",serial_no="%s"`,
		c.mchID, nonce, base64.StdEncoding.EncodeToString(sig), ts, c.mchSerialNo), nil
}

// doRequest 发起 API v3 请求并返回响应体；非 2xx 时返回带状态码的错误。
func (c *WechatPayClient) doRequest(method, urlPath, queryString, body string) ([]byte, error) {
	auth, err := c.sign(method, urlPath, queryString, body)
	if err != nil {
		return nil, err
	}
	url := wechatPayBaseURL + urlPath
	if queryString != "" {
		url += "?" + queryString
	}
	req, err := http.NewRequest(method, url, bytes.NewReader([]byte(body)))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", auth)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("微信支付 API 请求失败: status=%d body=%s", resp.StatusCode, string(respBody))
	}
	return respBody, nil
}

// orderBody 构造统一下单请求体（Native/H5 共用，H5 追加 scene_info）
func (c *WechatPayClient) orderBody(outTradeNo string, amount float64, desc string, sceneInfo map[string]interface{}) (string, error) {
	body := map[string]interface{}{
		"appid":        c.appID,
		"mchid":        c.mchID,
		"description":  desc,
		"out_trade_no": outTradeNo,
		"notify_url":   wechatPayNotifyURL(),
		"amount": map[string]interface{}{
			"total":    int64(amount * 100),
			"currency": "CNY",
		},
	}
	if sceneInfo != nil {
		body["scene_info"] = sceneInfo
	}
	b, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CreateNativeOrder 微信 Native 下单，返回支付二维码内容（code_url）。
func (c *WechatPayClient) CreateNativeOrder(outTradeNo string, amount float64, desc string) (string, error) {
	body, err := c.orderBody(outTradeNo, amount, desc, nil)
	if err != nil {
		return "", err
	}
	respBody, err := c.doRequest(http.MethodPost, "/v3/pay/transactions/native", "", body)
	if err != nil {
		return "", err
	}
	var r struct {
		CodeURL string `json:"code_url"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return "", fmt.Errorf("解析 Native 下单响应失败: %w", err)
	}
	if r.CodeURL == "" {
		return "", fmt.Errorf("微信 Native 下单未返回 code_url")
	}
	return r.CodeURL, nil
}

// CreateH5Order 微信 H5 下单，返回移动端拉起微信收银台的地址（h5_url）。
func (c *WechatPayClient) CreateH5Order(outTradeNo string, amount float64, desc, clientIP string) (string, error) {
	body, err := c.orderBody(outTradeNo, amount, desc, map[string]interface{}{
		"payer_client_ip": clientIP,
		"h5_info":         map[string]interface{}{"type": "Wap"},
	})
	if err != nil {
		return "", err
	}
	respBody, err := c.doRequest(http.MethodPost, "/v3/pay/transactions/h5", "", body)
	if err != nil {
		return "", err
	}
	var r struct {
		H5URL string `json:"h5_url"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return "", fmt.Errorf("解析 H5 下单响应失败: %w", err)
	}
	if r.H5URL == "" {
		return "", fmt.Errorf("微信 H5 下单未返回 h5_url")
	}
	return r.H5URL, nil
}

// QueryOrder 查询微信支付订单状态，返回交易状态与渠道交易号。
// trade_state：SUCCESS-支付成功 CLOSED-已关闭 NOTPAY-未支付 USERPAYING-支付中 PAYERROR-支付失败 REFUND-转入退款。
func (c *WechatPayClient) QueryOrder(outTradeNo string) (tradeState, transactionID string, err error) {
	path := "/v3/pay/transactions/out-trade-no/" + outTradeNo
	query := "mchid=" + c.mchID
	respBody, err := c.doRequest(http.MethodGet, path, query, "")
	if err != nil {
		return "", "", err
	}
	var r struct {
		TradeState    string `json:"trade_state"`
		TransactionID string `json:"transaction_id"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return "", "", fmt.Errorf("解析微信订单查询响应失败: %w", err)
	}
	return r.TradeState, r.TransactionID, nil
}

// wechatCallbackResource 微信支付回调通知中的加密资源（resource 字段）
type wechatCallbackResource struct {
	Ciphertext     string `json:"ciphertext"`
	Nonce          string `json:"nonce"`
	AssociatedData string `json:"associated_data"`
}

// WechatCallbackPayload 微信支付回调通知解密后的业务数据
type WechatCallbackPayload struct {
	OutTradeNo    string `json:"out_trade_no"`
	TransactionID string `json:"transaction_id"`
	TradeType     string `json:"trade_type"`
	TradeState    string `json:"trade_state"`
	Amount        struct {
		Total int64 `json:"total"`
	} `json:"amount"`
}

// CloseOrder 关闭微信支付订单（API v3 关单），撤回未支付交易使支付链接失效。
// 订单在微信侧不存在（404 ORDER_NOT_EXIST）时返回错误，由调用方记录日志（不影响本地关闭）。
func (c *WechatPayClient) CloseOrder(outTradeNo string) error {
	body := fmt.Sprintf(`{"mchid":"%s"}`, c.mchID)
	path := "/v3/pay/transactions/out-trade-no/" + outTradeNo + "/close"
	_, err := c.doRequest("POST", path, "", body)
	return err
}

// Refund 微信支付退款（API v3 POST /v3/refund/domestic/refunds），原路退回用户支付账户。
// outTradeNo 原支付单号；outRefundNo 商户退款单号（幂等，需唯一）；refundAmount 本次退款金额；
// totalAmount 原订单总金额（元）。退款受理后异步到账，接口返回 CLOSED/ABNORMAL 视为退款失败。
func (c *WechatPayClient) Refund(outTradeNo, outRefundNo string, refundAmount, totalAmount float64) error {
	body := map[string]interface{}{
		"out_trade_no":  outTradeNo,
		"out_refund_no": outRefundNo,
		"amount": map[string]interface{}{
			"refund":   int64(refundAmount * 100),
			"total":    int64(totalAmount * 100),
			"currency": "CNY",
		},
	}
	b, err := json.Marshal(body)
	if err != nil {
		return err
	}
	respBody, err := c.doRequest(http.MethodPost, "/v3/refund/domestic/refunds", "", string(b))
	if err != nil {
		return err
	}
	var r struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return fmt.Errorf("解析微信退款响应失败: %w", err)
	}
	switch r.Status {
	case "SUCCESS", "PROCESSING":
		return nil
	default:
		return fmt.Errorf("微信退款失败: status=%s", r.Status)
	}
}

// VerifyNotify 校验微信支付回调通知签名并解密业务数据：
//  1. 使用微信支付公钥验证 Wechatpay-Signature（报文串 timestamp\nnonce\nbody\n）；
//  2. 通过后用 APIv3 密钥对 resource 做 AES-256-GCM 解密。
func (c *WechatPayClient) VerifyNotify(headers map[string]string, body []byte) (*WechatCallbackPayload, error) {
	timestamp := headers["wechatpay-timestamp"]
	nonce := headers["wechatpay-nonce"]
	signature := headers["wechatpay-signature"]
	if timestamp == "" || nonce == "" || signature == "" {
		return nil, errors.New("回调缺少验签头")
	}

	message := fmt.Sprintf("%s\n%s\n%s\n", timestamp, nonce, string(body))
	hash := sha256.Sum256([]byte(message))
	sigBytes, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return nil, fmt.Errorf("回调签名 base64 解码失败: %w", err)
	}
	if err := rsa.VerifyPKCS1v15(c.publicKey, crypto.SHA256, hash[:], sigBytes); err != nil {
		return nil, errors.New("微信支付回调验签失败")
	}

	var notif struct {
		Resource wechatCallbackResource `json:"resource"`
	}
	if err := json.Unmarshal(body, &notif); err != nil {
		return nil, fmt.Errorf("解析回调通知失败: %w", err)
	}
	plain, err := c.decryptResource(&notif.Resource)
	if err != nil {
		return nil, err
	}
	var payload WechatCallbackPayload
	if err := json.Unmarshal(plain, &payload); err != nil {
		return nil, fmt.Errorf("解析回调业务数据失败: %w", err)
	}
	return &payload, nil
}

// decryptResource 用 APIv3 密钥解密回调 resource（AES-256-GCM）
func (c *WechatPayClient) decryptResource(res *wechatCallbackResource) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(res.Ciphertext)
	if err != nil {
		return nil, fmt.Errorf("回调密文 base64 解码失败: %w", err)
	}
	block, err := aes.NewCipher(c.apiV3Key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("回调密文长度非法")
	}
	plain, err := gcm.Open(nil, []byte(res.Nonce), ciphertext, []byte(res.AssociatedData))
	if err != nil {
		return nil, fmt.Errorf("回调解密失败: %w", err)
	}
	return plain, nil
}

// randomHex 生成指定字节数的随机十六进制字符串（用于 nonce）
func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
