package upstream

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"oemrpa/internal/site"
)

const (
	alipayDefaultGateway = "https://openapi.alipay.com/gateway.do"
)

// alipayNotifyURL 异步通知地址（指向平台 API 域回调路由，仅平台与支付宝可见）
func alipayNotifyURL() string {
	return site.Platform().APIBase() + "/v1/callback/alipay"
}

// alipayReturnBase 同步跳转地址默认前缀（控制台支付详情页 /payment/{pay_order_no}）
func alipayReturnBase() string {
	return site.Platform().ConsoleBase() + "/payment/"
}

// AlipayClient 支付宝开放平台支付客户端（电脑网站支付 alipay.trade.page.pay，RSA2 签名）
type AlipayClient struct {
	AppID      string
	PrivateKey *rsa.PrivateKey
	PublicKey  *rsa.PublicKey
}

// NewAlipayClient 创建支付宝支付客户端；未配置 AppID 时返回 (nil, nil)
func NewAlipayClient(appID, privateKeyPEM, publicKeyPEM string) (*AlipayClient, error) {
	if appID == "" {
		return nil, nil
	}
	priv, err := parseRSAPrivateKey(privateKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("解析支付宝应用私钥失败: %w", err)
	}
	pub, err := parseRSAPublicKey(publicKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("解析支付宝公钥失败: %w", err)
	}
	return &AlipayClient{
		AppID:      appID,
		PrivateKey: priv,
		PublicKey:  pub,
	}, nil
}

// BuildPagePayURL 构建电脑网站支付跳转链接（浏览器 GET 访问即可拉起支付宝收银台）。
// returnBase 为同步跳转地址前缀（如 https://console.example.com/payment/），为空时使用平台默认控制台地址。
func (c *AlipayClient) BuildPagePayURL(outTradeNo string, amount float64, subject, returnBase string) (string, error) {
	if returnBase == "" {
		returnBase = alipayReturnBase()
	}
	bizContent, err := json.Marshal(map[string]string{
		"out_trade_no": outTradeNo,
		"product_code": "FAST_INSTANT_TRADE_PAY",
		"total_amount": fmt.Sprintf("%.2f", amount),
		"subject":      subject,
	})
	if err != nil {
		return "", err
	}

	params := map[string]string{
		"app_id":      c.AppID,
		"method":      "alipay.trade.page.pay",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"notify_url":  alipayNotifyURL(),
		"return_url":  returnBase + outTradeNo,
		"biz_content": string(bizContent),
	}

	sign, err := c.Sign(params)
	if err != nil {
		return "", err
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("sign", sign)

	return alipayDefaultGateway + "?" + values.Encode(), nil
}

// AlipayTradeQuery 支付宝订单查询响应（alipay.trade.query）
type AlipayTradeQuery struct {
	AlipayTradeQueryResponse struct {
		Code        string `json:"code"`
		Msg         string `json:"msg"`
		SubCode     string `json:"sub_code"`
		SubMsg      string `json:"sub_msg"`
		OutTradeNo  string `json:"out_trade_no"`
		TradeNo     string `json:"trade_no"`
		TradeStatus string `json:"trade_status"`
		TotalAmount string `json:"total_amount"`
	} `json:"alipay_trade_query_response"`
	Sign string `json:"sign"`
}

// QueryOrder 查询支付宝订单状态（alipay.trade.query），用于支付对账与超时补单
func (c *AlipayClient) QueryOrder(outTradeNo string) (*AlipayTradeQuery, error) {
	bizContent := fmt.Sprintf(`{"out_trade_no":"%s"}`, outTradeNo)
	params := map[string]string{
		"app_id":      c.AppID,
		"method":      "alipay.trade.query",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": bizContent,
	}
	sign, err := c.Sign(params)
	if err != nil {
		return nil, err
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("sign", sign)

	resp, err := http.PostForm(alipayDefaultGateway, values)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result AlipayTradeQuery
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("解析支付宝订单查询响应失败: %w", err)
	}
	// 业务失败（如 ACQ.TRADE_NOT_EXIST 交易不存在）也原样返回，由调用方按 sub_code 处理
	return &result, nil
}

// CloseOrder 关闭支付宝订单（alipay.trade.close），撤回未支付交易使收银台/支付链接失效。
// 支付宝侧不存在该交易（ACQ.TRADE_NOT_EXIST，用户从未扫码生成交易）视为已无交易，不报错。
func (c *AlipayClient) CloseOrder(outTradeNo string) error {
	bizContent := fmt.Sprintf(`{"out_trade_no":"%s"}`, outTradeNo)
	params := map[string]string{
		"app_id":      c.AppID,
		"method":      "alipay.trade.close",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": bizContent,
	}
	sign, err := c.Sign(params)
	if err != nil {
		return err
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("sign", sign)

	resp, err := http.PostForm(alipayDefaultGateway, values)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result struct {
		AlipayTradeCloseResponse struct {
			Code    string `json:"code"`
			SubCode string `json:"sub_code"`
			Msg     string `json:"msg"`
			SubMsg  string `json:"sub_msg"`
		} `json:"alipay_trade_close_response"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return fmt.Errorf("解析支付宝关单响应失败: %w", err)
	}
	r := result.AlipayTradeCloseResponse
	if r.Code == "10000" || r.SubCode == "ACQ.TRADE_NOT_EXIST" {
		return nil
	}
	return fmt.Errorf("支付宝关单失败: code=%s, sub_code=%s, msg=%s", r.Code, r.SubCode, r.Msg)
}

// RefundOrder 支付宝退款（alipay.trade.refund），原路退回用户支付账户。
// outTradeNo 原支付单号；refundAmount 本次退款金额；outRequestNo 退款请求号（部分退款场景幂等，需唯一）。
// 单笔支付单累计退款不可超过订单金额，超出时支付宝返回业务错误。
func (c *AlipayClient) RefundOrder(outTradeNo string, refundAmount float64, outRequestNo string) error {
	bizContent := fmt.Sprintf(`{"out_trade_no":"%s","refund_amount":"%.2f","out_request_no":"%s"}`, outTradeNo, refundAmount, outRequestNo)
	params := map[string]string{
		"app_id":      c.AppID,
		"method":      "alipay.trade.refund",
		"format":      "JSON",
		"charset":     "utf-8",
		"sign_type":   "RSA2",
		"timestamp":   time.Now().Format("2006-01-02 15:04:05"),
		"version":     "1.0",
		"biz_content": bizContent,
	}
	sign, err := c.Sign(params)
	if err != nil {
		return err
	}

	values := url.Values{}
	for k, v := range params {
		values.Set(k, v)
	}
	values.Set("sign", sign)

	resp, err := http.PostForm(alipayDefaultGateway, values)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var result struct {
		AlipayTradeRefundResponse struct {
			Code    string `json:"code"`
			SubCode string `json:"sub_code"`
			Msg     string `json:"msg"`
			SubMsg  string `json:"sub_msg"`
		} `json:"alipay_trade_refund_response"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return fmt.Errorf("解析支付宝退款响应失败: %w", err)
	}
	r := result.AlipayTradeRefundResponse
	if r.Code == "10000" {
		return nil
	}
	switch r.SubCode {
	case "ACQ.SELLER_BALANCE_NOT_ENOUGH":
		// 商户（平台）支付宝账户可用余额不足，退款无法执行，需商户充值后重试
		return fmt.Errorf("支付宝商户账户余额不足，无法完成退款，请稍后重试或联系客服")
	default:
		return fmt.Errorf("支付宝退款失败: code=%s, sub_code=%s, msg=%s", r.Code, r.SubCode, r.Msg)
	}
}

// Sign 对参数按 key 升序拼接 key=value（以 & 连接），使用应用私钥做 RSA2 签名
func (c *AlipayClient) Sign(params map[string]string) (string, error) {
	content := buildSignContent(params)
	hash := sha256.Sum256([]byte(content))
	sig, err := rsa.SignPKCS1v15(rand.Reader, c.PrivateKey, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sig), nil
}

// VerifyNotify 验证异步通知签名（排除 sign 与 sign_type 字段）
func (c *AlipayClient) VerifyNotify(params map[string]string) bool {
	sign := params["sign"]
	if sign == "" {
		return false
	}
	sigBytes, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false
	}

	filtered := make(map[string]string)
	for k, v := range params {
		if k == "sign" || k == "sign_type" {
			continue
		}
		filtered[k] = v
	}
	hash := sha256.Sum256([]byte(buildSignContent(filtered)))
	return rsa.VerifyPKCS1v15(c.PublicKey, crypto.SHA256, hash[:], sigBytes) == nil
}

// buildSignContent 按 key 升序拼接 key=value，以 & 连接（剔除空值与 sign）
func buildSignContent(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k, v := range params {
		if k == "sign" || v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteString("&")
		}
		sb.WriteString(k)
		sb.WriteString("=")
		sb.WriteString(params[k])
	}
	return sb.String()
}

// parseRSAPrivateKey 解析 PEM 私钥（支持 PKCS1 与 PKCS8 格式）
func parseRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("pem 解码失败")
	}
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PrivateKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("不是 RSA 私钥")
	}
	return nil, errors.New("不支持的私钥格式")
}

// parseRSAPublicKey 解析 PEM 公钥（支持 PKCS1 与 PKIX 格式）
func parseRSAPublicKey(pemStr string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, errors.New("pem 解码失败")
	}
	if key, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if rsaKey, ok := key.(*rsa.PublicKey); ok {
			return rsaKey, nil
		}
		return nil, errors.New("不是 RSA 公钥")
	}
	return nil, errors.New("不支持的公钥格式")
}
