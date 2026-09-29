package starloft

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// canonicalSign 对字段按 key 字典序拼接为 k=v&k=v...（不做 URL 编码）后取
// HMAC-SHA256(APISecret, canonical) 的十六进制小写；与平台侧推送签名算法一致。
func (c *Client) canonicalSign(fields map[string]string) string {
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
	return hex.EncodeToString(mac.Sum(nil))
}

// signEqual 恒定时间比较签名
func signEqual(expected, received string) bool {
	if expected == "" || received == "" {
		return false
	}
	return hmac.Equal([]byte(expected), []byte(received))
}

// VerifySmsReceiptSign 校验短信回执推送签名：
// 字段 biz_no / code_desc / message_sid / phone / resp_code / status
func (c *Client) VerifySmsReceiptSign(body []byte) bool {
	if !c.Available() {
		return false
	}
	var p struct {
		BizNo      string `json:"biz_no"`
		CodeDesc   string `json:"code_desc"`
		MessageSid string `json:"message_sid"`
		Phone      string `json:"phone"`
		RespCode   string `json:"resp_code"`
		Status     string `json:"status"`
		Sign       string `json:"sign"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return false
	}
	return signEqual(c.canonicalSign(map[string]string{
		"biz_no":      p.BizNo,
		"code_desc":   p.CodeDesc,
		"message_sid": p.MessageSid,
		"phone":       p.Phone,
		"resp_code":   p.RespCode,
		"status":      p.Status,
	}), p.Sign)
}

// VerifySmsReplySign 校验短信回复推送签名：
// 字段 biz_no / content_down / content_up / message_sid / phone / status
func (c *Client) VerifySmsReplySign(body []byte) bool {
	if !c.Available() {
		return false
	}
	var p struct {
		BizNo       string `json:"biz_no"`
		ContentDown string `json:"content_down"`
		ContentUp   string `json:"content_up"`
		MessageSid  string `json:"message_sid"`
		Phone       string `json:"phone"`
		Status      string `json:"status"`
		Sign        string `json:"sign"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return false
	}
	return signEqual(c.canonicalSign(map[string]string{
		"biz_no":       p.BizNo,
		"content_down": p.ContentDown,
		"content_up":   p.ContentUp,
		"message_sid":  p.MessageSid,
		"phone":        p.Phone,
		"status":       p.Status,
	}), p.Sign)
}

// VerifySmsStatusSign 校验签名/模板审核状态推送签名：
// 字段 biz_type / reason / record_id / status / up_id
func (c *Client) VerifySmsStatusSign(body []byte) bool {
	if !c.Available() {
		return false
	}
	var p struct {
		BizType  string `json:"biz_type"`
		Reason   string `json:"reason"`
		RecordID int64  `json:"record_id"`
		Status   int    `json:"status"`
		UpID     string `json:"up_id"`
		Sign     string `json:"sign"`
	}
	if err := json.Unmarshal(body, &p); err != nil {
		return false
	}
	return signEqual(c.canonicalSign(map[string]string{
		"biz_type":  p.BizType,
		"reason":    p.Reason,
		"record_id": strconv.FormatInt(p.RecordID, 10),
		"status":    strconv.Itoa(p.Status),
		"up_id":     p.UpID,
	}), p.Sign)
}

var _ interface {
	VerifySmsReceiptSign(body []byte) bool
	VerifySmsReplySign(body []byte) bool
	VerifySmsStatusSign(body []byte) bool
} = (*Client)(nil)