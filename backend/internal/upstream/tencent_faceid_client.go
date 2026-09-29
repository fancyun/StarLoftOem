package upstream

import (
	"encoding/json"
	"fmt"
)

// TencentFaceIdClient 腾讯云人脸核身（慧眼）客户端，用于平台自用「法人扫脸」。
// 接入流程：
//   - DetectAuth 获取 BizToken 与 H5 核身地址 Url；
//   - 浏览器访问 Url+"&token="+BizToken 完成活体人脸核身；
//   - GetDetectInfoEnhanced 用 BizToken 查询核身结果（DetectInfoText.ErrCode==0 为通过）。
type TencentFaceIdClient struct {
	client *TencentClient
	ruleId string
}

// NewTencentFaceIdClient 创建人脸核身客户端。
func NewTencentFaceIdClient(secretID, secretKey, region, ruleId string) *TencentFaceIdClient {
	return &TencentFaceIdClient{
		client: NewTencentClient(secretID, secretKey, region),
		ruleId: ruleId,
	}
}

// DetectAuthResult DetectAuth 返回。
type DetectAuthResult struct {
	BizToken string // 核身流程标识
	Url      string // H5 核身地址（需拼接 &token=BizToken）
}

// DetectAuth 发起实名核身鉴权，返回核身地址与 BizToken。
func (c *TencentFaceIdClient) DetectAuth(name, idCard, redirectURL, extra string) (*DetectAuthResult, error) {
	if c.ruleId == "" {
		return nil, fmt.Errorf("人脸核身 RuleId 未配置")
	}
	body := map[string]interface{}{
		"RuleId":      c.ruleId,
		"Name":        name,
		"IdCard":      idCard,
		"RedirectUrl": redirectURL,
		"Extra":       extra,
	}
	respBody, err := c.client.Call("faceid", "DetectAuth", "2018-03-01", body)
	if err != nil {
		return nil, err
	}
	if err := TencentError(respBody); err != nil {
		return nil, err
	}
	var r struct {
		Response struct {
			BizToken  string `json:"BizToken"`
			Url       string `json:"Url"`
			RequestId string `json:"RequestId"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}
	if r.Response.BizToken == "" {
		return nil, fmt.Errorf("人脸核身 DetectAuth 未返回 BizToken")
	}
	return &DetectAuthResult{BizToken: r.Response.BizToken, Url: r.Response.Url}, nil
}

// DetectResult GetDetectInfoEnhanced 返回的核身结果（取 DetectInfoText）。
type DetectResult struct {
	ErrCode     int // 0-通过，其余失败
	ErrMsg      string
	IdCard      string
	Name        string
	Similarity  float64
	Description string
}

// GetDetectInfo 用 BizToken 查询核身结果。
func (c *TencentFaceIdClient) GetDetectInfo(bizToken string) (*DetectResult, error) {
	if c.ruleId == "" {
		return nil, fmt.Errorf("人脸核身 RuleId 未配置")
	}
	respBody, err := c.client.Call("faceid", "GetDetectInfoEnhanced", "2018-03-01", map[string]interface{}{
		"BizToken": bizToken,
		"RuleId":   c.ruleId,
	})
	if err != nil {
		return nil, err
	}
	if err := TencentError(respBody); err != nil {
		return nil, err
	}
	var r struct {
		Response struct {
			DetectInfoText struct {
				ErrCode      int     `json:"ErrCode"`
				ErrMsg       string  `json:"ErrMsg"`
				IdCard       string  `json:"IdCard"`
				Name         string  `json:"Name"`
				Sim          float64 `json:"Sim"`
				IsNeedCharge bool    `json:"IsNeedCharge"`
				Description  string  `json:"Description"`
			} `json:"DetectInfoText"`
			RequestId string `json:"RequestId"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}
	return &DetectResult{
		ErrCode:     r.Response.DetectInfoText.ErrCode,
		ErrMsg:      r.Response.DetectInfoText.ErrMsg,
		IdCard:      r.Response.DetectInfoText.IdCard,
		Name:        r.Response.DetectInfoText.Name,
		Similarity:  r.Response.DetectInfoText.Sim,
		Description: r.Response.DetectInfoText.Description,
	}, nil
}
