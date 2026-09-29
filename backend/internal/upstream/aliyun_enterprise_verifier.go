package upstream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"oemrpa/internal/logstore"
)

// AliyunEnterpriseVerifier 阿里云云市场「企业工商四要素核验」provider。
// 鉴权方式为云市场 AppCode（请求头 Authorization: APPCODE xxx）；
// 接口地址由后台「系统设置」的 ALIYUN_ENTERPRISE_VERIFY_URL 提供（订阅商品后获得），
// 上送字段名按云市场常见约定（companyName / creditCode / legalPerson / legalIdCard），
// 返回结构因商品而异，本实现按常见字段（data.matched / result / verifyResult 等）尽力解析，
// 若所订阅商品的字段不同，仅需调整本文件的请求参数与解析字段，不影响业务层。
type AliyunEnterpriseVerifier struct {
	endpoint string
	appCode  string
	client   *http.Client
}

// NewAliyunEnterpriseVerifier 创建阿里云企业四要素核验 provider
func NewAliyunEnterpriseVerifier(endpoint, appCode string) *AliyunEnterpriseVerifier {
	return &AliyunEnterpriseVerifier{
		endpoint: strings.TrimSpace(endpoint),
		appCode:  strings.TrimSpace(appCode),
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

// Verify 核验企业名称、统一社会信用代码、法人姓名、法人身份证号四要素一致性
func (v *AliyunEnterpriseVerifier) Verify(companyName, creditCode, legalName, legalIDCard string) (*EnterpriseVerifyResult, error) {
	if v.endpoint == "" || v.appCode == "" {
		return nil, fmt.Errorf("阿里云企业四要素核验未配置（接口地址或 AppCode 缺失）")
	}

	form := url.Values{}
	form.Set("companyName", companyName)
	form.Set("creditCode", creditCode)
	form.Set("legalPerson", legalName)
	form.Set("legalIdCard", legalIDCard)

	req, err := http.NewRequest(http.MethodPost, v.endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("构建核验请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")
	req.Header.Set("Authorization", "APPCODE "+v.appCode)

	resp, err := v.client.Do(req)
	if err != nil {
		logstore.RecordSysCall("request", "aliyun-enterprise4", "", 0, "", "", err.Error(), 0)
		return nil, fmt.Errorf("调用核验接口失败: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取核验响应失败: %w", err)
	}
	// 请求体含企业名与法人证件信息，不落日志；仅记录响应
	status := 1
	if resp.StatusCode >= 400 {
		status = 0
	}
	logstore.RecordSysCall("request", "aliyun-enterprise4", "", 0, "", "", string(raw), status)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("核验接口返回异常: http %d", resp.StatusCode)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("解析核验响应失败: %w", err)
	}

	matched, message := parseEnterpriseVerifyResult(body)
	return &EnterpriseVerifyResult{Matched: matched, Message: message, RawData: string(raw)}, nil
}

// parseEnterpriseVerifyResult 从上游返回中尽力取出「四要素是否一致」的结论与说明：
// 依次尝试 data 子对象与顶层对象的 matched / result / verifyResult / isMatch 字段。
func parseEnterpriseVerifyResult(body map[string]interface{}) (bool, string) {
	if data, ok := body["data"].(map[string]interface{}); ok {
		if matched, ok := pickBool(data, "matched", "result", "verifyResult", "isMatch", "is_match"); ok {
			return matched, pickString(data, "msg", "message", "reason", "desc")
		}
	}
	if matched, ok := pickBool(body, "matched", "result", "verifyResult", "isMatch", "is_match"); ok {
		return matched, pickString(body, "msg", "message", "reason", "desc")
	}
	return false, "上游返回未包含可识别的核验结论，请核对该商品的返回字段"
}

// pickBool 按候选键名取出布尔结论（兼容 true/false、"true"/"false"、1/0）
func pickBool(m map[string]interface{}, keys ...string) (bool, bool) {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch t := v.(type) {
		case bool:
			return t, true
		case string:
			s := strings.ToLower(strings.TrimSpace(t))
			if s == "true" {
				return true, true
			}
			if s == "false" {
				return false, true
			}
		case float64:
			return t != 0, true
		}
	}
	return false, false
}

// pickString 按候选键名取出说明文本
func pickString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k].(string); ok && strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

var _ EnterpriseVerifier = (*AliyunEnterpriseVerifier)(nil)