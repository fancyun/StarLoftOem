package service

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"oemrpa/internal/logstore"
)

// CaptchaProvider 人机验证码 provider（腾讯天御 / 极验 / 阿里云行为验证码，后台可切换）。
// 各通道前端渲染参数与服务端校验字段不同，统一以 payload（前端提交的验证参数）透传。
// 未配置对应凭据时一律放行（与历史行为一致，避免本地/演示环境阻断登录注册）；生产环境必须配置。
type CaptchaProvider interface {
	// Name 通道标识（tencent / geetest / aliyun）
	Name() string
	// Verify 服务端二次校验
	Verify(payload map[string]string, userIP string) error
	// FrontConfig 下发给前端的渲染参数
	FrontConfig() map[string]interface{}
}

// 验证码通道标识（与后台配置 CAPTCHA_PROVIDER 的取值一致）
const (
	CaptchaProviderNameTencent = "tencent"
	CaptchaProviderNameGeetest = "geetest"
	CaptchaProviderNameAliyun  = "aliyun"
)

// 前端验证参数键（各通道提交的字段名）
const (
	CaptchaFieldTicket     = "ticket"               // 腾讯：票据
	CaptchaFieldRandStr    = "randstr"              // 腾讯：随机串
	CaptchaFieldLotNumber  = "lot_number"           // 极验：批次号
	CaptchaFieldCaptchaOut = "captcha_output"       // 极验：验证输出
	CaptchaFieldPassToken  = "pass_token"           // 极验：通过令牌
	CaptchaFieldGenTime    = "gen_time"             // 极验：生成时间
	CaptchaFieldVerifyPara = "captcha_verify_param" // 阿里云：验证参数
)

// ===== 腾讯天御 =====

// tencentCaptchaProvider 腾讯天御验证码 provider（包装既有 CaptchaService）
type tencentCaptchaProvider struct {
	svc   *CaptchaService
	appID string
}

// NewTencentCaptchaProvider 创建腾讯天御验证码 provider
func NewTencentCaptchaProvider(svc *CaptchaService, appID string) CaptchaProvider {
	return &tencentCaptchaProvider{svc: svc, appID: appID}
}

func (p *tencentCaptchaProvider) Name() string { return CaptchaProviderNameTencent }

func (p *tencentCaptchaProvider) Verify(payload map[string]string, userIP string) error {
	return p.svc.VerifyCaptcha(payload[CaptchaFieldTicket], payload[CaptchaFieldRandStr], userIP)
}

func (p *tencentCaptchaProvider) FrontConfig() map[string]interface{} {
	return map[string]interface{}{"provider": p.Name(), "app_id": p.appID}
}

// ===== 极验行为验证码 =====

// geetestCaptchaProvider 极验行为验证码 provider（v4 服务端二次校验）
type geetestCaptchaProvider struct {
	captchaID  string
	captchaKey string
	endpoint   string
	client     *http.Client
}

// NewGeetestCaptchaProvider 创建极验验证码 provider
func NewGeetestCaptchaProvider(captchaID, captchaKey string) CaptchaProvider {
	return &geetestCaptchaProvider{
		captchaID:  strings.TrimSpace(captchaID),
		captchaKey: strings.TrimSpace(captchaKey),
		endpoint:   "https://gcaptcha4.geetest.com/validate",
		client:     &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *geetestCaptchaProvider) Name() string { return CaptchaProviderNameGeetest }

func (p *geetestCaptchaProvider) Verify(payload map[string]string, userIP string) error {
	// 未配置凭据时放行（与腾讯天御口径一致）
	if p.captchaID == "" || p.captchaKey == "" {
		return nil
	}
	lotNumber := payload[CaptchaFieldLotNumber]
	if lotNumber == "" {
		return fmt.Errorf("验证码参数缺失")
	}

	// sign_token = HMAC-SHA256(CaptchaKey, lot_number) 十六进制小写
	mac := hmac.New(sha256.New, []byte(p.captchaKey))
	mac.Write([]byte(lotNumber))
	signToken := hex.EncodeToString(mac.Sum(nil))

	form := url.Values{}
	form.Set("lot_number", lotNumber)
	form.Set("captcha_output", payload[CaptchaFieldCaptchaOut])
	form.Set("pass_token", payload[CaptchaFieldPassToken])
	form.Set("gen_time", payload[CaptchaFieldGenTime])
	form.Set("sign_token", signToken)

	endpoint := p.endpoint + "?captcha_id=" + url.QueryEscape(p.captchaID)
	resp, err := p.client.Post(endpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		logstore.RecordSysCall("request", "geetest-captcha", "", 0, "", "", err.Error(), 0)
		return fmt.Errorf("验证码校验请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取验证码校验响应失败: %w", err)
	}
	var out struct {
		Result string `json:"result"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		logstore.RecordSysCall("request", "geetest-captcha", "", 0, "", "", string(raw), 0)
		return fmt.Errorf("解析验证码校验响应失败: %w", err)
	}
	logstore.RecordSysCall("request", "geetest-captcha", "", 0, "", "", string(raw), 1)
	if out.Result != "success" {
		return fmt.Errorf("验证码校验未通过: %s", out.Reason)
	}
	return nil
}

func (p *geetestCaptchaProvider) FrontConfig() map[string]interface{} {
	return map[string]interface{}{"provider": p.Name(), "captcha_id": p.captchaID}
}

// ===== 阿里云行为验证码 =====

// aliyunCaptchaProvider 阿里云验证码 2.0 provider（VerifyIntelligentCaptcha，RPC 风格 HMAC-SHA1 签名）
type aliyunCaptchaProvider struct {
	accessKeyID     string
	accessKeySecret string
	sceneID         string
	endpoint        string
	client          *http.Client
}

// NewAliyunCaptchaProvider 创建阿里云验证码 provider
func NewAliyunCaptchaProvider(accessKeyID, accessKeySecret, sceneID string) CaptchaProvider {
	return &aliyunCaptchaProvider{
		accessKeyID:     strings.TrimSpace(accessKeyID),
		accessKeySecret: strings.TrimSpace(accessKeySecret),
		sceneID:         strings.TrimSpace(sceneID),
		endpoint:        "https://captcha.cn-shanghai.aliyuncs.com/",
		client:          &http.Client{Timeout: 10 * time.Second},
	}
}

func (p *aliyunCaptchaProvider) Name() string { return CaptchaProviderNameAliyun }

func (p *aliyunCaptchaProvider) Verify(payload map[string]string, userIP string) error {
	// 未配置凭据时放行（与腾讯天御口径一致）
	if p.accessKeyID == "" || p.accessKeySecret == "" {
		return nil
	}
	verifyParam := payload[CaptchaFieldVerifyPara]
	if verifyParam == "" {
		return fmt.Errorf("验证码参数缺失")
	}

	params := map[string]string{
		"Action":             "VerifyIntelligentCaptcha",
		"Version":            "2023-03-05",
		"Format":             "JSON",
		"RegionId":           "cn-shanghai",
		"AccessKeyId":        p.accessKeyID,
		"SignatureMethod":    "HMAC-SHA1",
		"SignatureVersion":   "1.0",
		"SignatureNonce":     strconv.FormatInt(time.Now().UnixNano(), 10),
		"Timestamp":          time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"CaptchaVerifyParam": verifyParam,
	}
	if p.sceneID != "" {
		params["SceneId"] = p.sceneID
	}
	params["Signature"] = aliyunRPCSignature("POST", params, p.accessKeySecret)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	resp, err := p.client.Post(p.endpoint, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
	if err != nil {
		logstore.RecordSysCall("request", "aliyun-captcha", "", 0, "", "", err.Error(), 0)
		return fmt.Errorf("验证码校验请求失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取验证码校验响应失败: %w", err)
	}
	var out struct {
		VerifyResult bool   `json:"VerifyResult"`
		VerifyCode   string `json:"VerifyCode"`
		VerifyMsg    string `json:"VerifyMsg"`
		Code         string `json:"Code"`
		Message      string `json:"Message"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		logstore.RecordSysCall("request", "aliyun-captcha", "", 0, "", "", string(raw), 0)
		return fmt.Errorf("解析验证码校验响应失败: %w", err)
	}
	status := 1
	if !out.VerifyResult {
		status = 0
	}
	logstore.RecordSysCall("request", "aliyun-captcha", "", 0, "", "", string(raw), status)
	if out.Code != "" {
		return fmt.Errorf("验证码校验失败: %s", out.Message)
	}
	if !out.VerifyResult {
		return fmt.Errorf("验证码校验未通过: %s", out.VerifyMsg)
	}
	return nil
}

func (p *aliyunCaptchaProvider) FrontConfig() map[string]interface{} {
	return map[string]interface{}{"provider": p.Name(), "scene_id": p.sceneID}
}

// aliyunPercentEncode 阿里云 RPC 签名要求的百分号编码（RFC3986：空格为 %20、* 为 %2A、~ 不编码）
func aliyunPercentEncode(s string) string {
	encoded := url.QueryEscape(s)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}

// aliyunRPCSignature 计算阿里云 RPC 风格接口签名（HMAC-SHA1，Base64）
func aliyunRPCSignature(method string, params map[string]string, accessKeySecret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		if k == "Signature" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, aliyunPercentEncode(k)+"="+aliyunPercentEncode(params[k]))
	}
	canonicalQuery := strings.Join(parts, "&")

	stringToSign := method + "&" + aliyunPercentEncode("/") + "&" + aliyunPercentEncode(canonicalQuery)
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}