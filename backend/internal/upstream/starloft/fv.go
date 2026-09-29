package starloft

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"

	"oemrpa/internal/model"
	"oemrpa/internal/upstream"
)

var _ upstream.FinAuthInterface = (*Client)(nil)

// fvPath 人脸核验发起路径：无源（fv_self）走 /v1/fv/self，其余走 /v1/fv/auth
func fvPath(product string) string {
	if product == model.ServiceFVSelf {
		return "/v1/fv/self"
	}
	return "/v1/fv/auth"
}

// tokenFromSiteURL 从平台返回的自站链接中取出核身令牌（?token=xxx）
func tokenFromSiteURL(siteURL string) string {
	u, err := url.Parse(siteURL)
	if err != nil {
		return ""
	}
	return u.Query().Get("token")
}

// GetToken 发起人脸核验：POST /v1/fv/auth 或 /v1/fv/self。
// 平台返回自站承接链接（含核身令牌）与平台业务号；本平台据此拼装自己的承接页链接，
// 承接页再 302 到平台承接页完成核身。
func (c *Client) GetToken(req *upstream.GetTokenRequest) (*upstream.GetTokenResponse, error) {
	payload := map[string]interface{}{
		"name":           req.IDCardName,
		"id_card":        req.IDCardNumber,
		"return_url":     req.ReturnURL,
		"notify_url":     req.NotifyURL,
		"biz_extra_data": req.BizNo, // 本平台业务号随之上送，平台回调时原样带回，用于精确定位订单
	}

	var out struct {
		BizNo       string `json:"biz_no"`
		SiteURL     string `json:"site_url"`
		ExpiredIn   int64  `json:"expired_in"`
		ExpiredTime int64  `json:"expired_time"`
	}
	if err := c.call(http.MethodPost, fvPath(req.Product), "fv-auth", req.BizNo, payload, &out); err != nil {
		return nil, err
	}
	if out.BizNo == "" {
		return nil, fmt.Errorf("StarLoft 平台未返回业务号")
	}
	token := tokenFromSiteURL(out.SiteURL)
	if token == "" {
		return nil, fmt.Errorf("StarLoft 平台未返回核身令牌")
	}
	return &upstream.GetTokenResponse{
		RequestID:   out.BizNo,
		Token:       token,
		BizID:       out.BizNo,
		ExpiredTime: out.ExpiredTime,
	}, nil
}

// GetResult 查询核验结果：POST /v1/fv/result（按平台业务号查询）。
// 结果码沿用上游口径（1000 成功；其余失败码见业务层映射）。
func (c *Client) GetResult(req *upstream.GetResultRequest) (*upstream.GetResultResponse, error) {
	if strings.TrimSpace(req.BizID) == "" {
		return nil, fmt.Errorf("业务号不能为空")
	}
	var out struct {
		BizNo         string `json:"biz_no"`
		Status        int    `json:"status"`
		ResultCode    string `json:"result_code"`
		ResultMessage string `json:"result_message"`
	}
	if err := c.call(http.MethodPost, "/v1/fv/result", "fv-result", req.BizID,
		map[string]interface{}{"biz_no": req.BizID}, &out); err != nil {
		return nil, err
	}

	// 平台已终结为「超时结束」或结果数据已销毁：结果永久不可取回，
	// 交由业务层按「未完成核身（不计费已退款）」终结，避免订单永远停在认证中。
	if out.Status == model.AuthStatusTimeout || strings.EqualFold(out.ResultCode, "DATA_DESTROYED") {
		return nil, fmt.Errorf("%w: %s", upstream.ErrFinAuthDataDestroyed, out.ResultMessage)
	}

	resp := &upstream.GetResultResponse{
		ResultCode:    upstream.FlexInt(parseResultCode(out.ResultCode)),
		ResultMessage: out.ResultMessage,
	}
	resp.BizInfo.BizID = out.BizNo
	resp.BizInfo.BizNo = out.BizNo
	return resp, nil
}

// parseResultCode 解析上游结果码字符串（非数字时返回 0，交由业务层按失败处理）
func parseResultCode(code string) int {
	n, err := strconv.Atoi(strings.TrimSpace(code))
	if err != nil {
		return 0
	}
	return n
}

// GetBestImg 领取活体最佳图：POST /v1/fv/best-img（返回 base64，平台侧限认证成功后 24 小时内、每单一次）
func (c *Client) GetBestImg(bizNo string) (string, error) {
	if strings.TrimSpace(bizNo) == "" {
		return "", fmt.Errorf("业务号不能为空")
	}
	var out struct {
		BestImg string `json:"best_img"`
	}
	if err := c.call(http.MethodPost, "/v1/fv/best-img", "fv-best-img", bizNo,
		map[string]interface{}{"biz_no": bizNo}, &out); err != nil {
		return "", err
	}
	if out.BestImg == "" {
		return "", fmt.Errorf("StarLoft 平台未返回活体图片")
	}
	return out.BestImg, nil
}

// GetMedia 下载认证媒体（照片/视频，base64）：POST /v1/fv/media
func (c *Client) GetMedia(bizNo string) (map[string]string, error) {
	if strings.TrimSpace(bizNo) == "" {
		return nil, fmt.Errorf("业务号不能为空")
	}
	var out struct {
		Media map[string]string `json:"media"`
	}
	if err := c.call(http.MethodPost, "/v1/fv/media", "fv-media", bizNo,
		map[string]interface{}{"biz_no": bizNo}, &out); err != nil {
		return nil, err
	}
	if out.Media == nil {
		out.Media = map[string]string{}
	}
	return out.Media, nil
}

// VerifySign 校验平台回调签名：对 biz_no/cost/result_code/result_message/status
// 按 key 字典序拼接为 k=v&... 再以 HMAC-SHA256(APISecret, canonical) 计算十六进制小写签名。
// cost 统一格式化为两位小数，与平台侧口径一致。
func (c *Client) VerifySign(jsonData, receivedSign string) bool {
	if receivedSign == "" || c.apiSecret == "" {
		return false
	}
	var payload struct {
		BizNo         string  `json:"biz_no"`
		Cost          float64 `json:"cost"`
		ResultCode    string  `json:"result_code"`
		ResultMessage string  `json:"result_message"`
		Status        int     `json:"status"`
	}
	if err := json.Unmarshal([]byte(jsonData), &payload); err != nil {
		return false
	}

	fields := map[string]string{
		"biz_no":         payload.BizNo,
		"cost":           strconv.FormatFloat(payload.Cost, 'f', 2, 64),
		"result_code":    payload.ResultCode,
		"result_message": payload.ResultMessage,
		"status":         strconv.Itoa(payload.Status),
	}
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(k)
		sb.WriteByte('=')
		sb.WriteString(fields[k])
	}

	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(sb.String()))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(receivedSign))
}