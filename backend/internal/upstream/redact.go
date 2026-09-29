package upstream

import "encoding/json"

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