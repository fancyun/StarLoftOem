package upstream

import (
	"encoding/json"
	"fmt"
)

// TencentOcrClient 腾讯云文字识别（OCR）客户端，用于平台自用「营业执照核验（企业四要素）」。
// 归属文字识别产品（ocr.tencentcloudapi.com），CAM 资源类型为 ocr，与慧眼（faceid）相互独立。
type TencentOcrClient struct {
	client *TencentClient
}

// NewTencentOcrClient 创建 OCR 客户端（复用腾讯云账号密钥，无需单独开通，CAM 授权 ocr 即可）。
func NewTencentOcrClient(secretID, secretKey, region string) *TencentOcrClient {
	return &TencentOcrClient{
		client: NewTencentClient(secretID, secretKey, region),
	}
}

// BizLicenseVerifyResult VerifyBizLicenseEnterprise4 返回的企业四要素核验结果。
type BizLicenseVerifyResult struct {
	StatusCode             int    // 请求状态：0-成功计费 1-系统异常不计费
	VerifyResult           int    // 验证结果：1-四要素完全匹配 0-不完全匹配
	IsCreditCodeConsistent bool   // 统一社会信用代码是否一致
	IsEntNameConsistent    bool   // 企业名称是否一致
	IsLrNameConsistent     bool   // 法人代表是否一致
	IsIdNumConsistent      bool   // 注册登记证件号码是否一致
	OperatingStatus        string // 经营状态（1-开业 3-注销 4-吊销 等）
	OperatingPeriod        string // 营业期限（yyyy-MM-dd/yyyy-MM-dd）
	RawData                string // 原始返回数据（JSON，落库用）
}

// VerifyBizLicenseEnterprise4 营业执照核验（企业四要素）：
// 比对企业名称、统一社会信用代码、法人姓名、注册登记证件号码（法人身份证号）四要素一致性。
func (c *TencentOcrClient) VerifyBizLicenseEnterprise4(entName, creditCode, lrName, idNum string) (*BizLicenseVerifyResult, error) {
	body := map[string]interface{}{
		"EntName":    entName,
		"CreditCode": creditCode,
		"LrName":     lrName,
		"IdNum":      idNum,
	}
	respBody, err := c.client.Call("ocr", "VerifyBizLicenseEnterprise4", "2018-11-19", body)
	if err != nil {
		return nil, err
	}
	if err := TencentError(respBody); err != nil {
		return nil, err
	}
	var r struct {
		Response struct {
			StatusCode             int    `json:"StatusCode"`
			VerifyResult           int    `json:"VerifyResult"`
			IsCreditCodeConsistent bool   `json:"IsCreditCodeConsistent"`
			IsEntNameConsistent    bool   `json:"IsEntNameConsistent"`
			IsLrNameConsistent     bool   `json:"IsLrNameConsistent"`
			IsIdNumConsistent      bool   `json:"IsIdNumConsistent"`
			OperatingStatus        string `json:"OperatingStatus"`
			OperatingPeriod        string `json:"OperatingPeriod"`
			RequestId              string `json:"RequestId"`
		} `json:"Response"`
	}
	if err := json.Unmarshal(respBody, &r); err != nil {
		return nil, fmt.Errorf("parse response failed: %w", err)
	}
	return &BizLicenseVerifyResult{
		StatusCode:             r.Response.StatusCode,
		VerifyResult:           r.Response.VerifyResult,
		IsCreditCodeConsistent: r.Response.IsCreditCodeConsistent,
		IsEntNameConsistent:    r.Response.IsEntNameConsistent,
		IsLrNameConsistent:     r.Response.IsLrNameConsistent,
		IsIdNumConsistent:      r.Response.IsIdNumConsistent,
		OperatingStatus:        r.Response.OperatingStatus,
		OperatingPeriod:        r.Response.OperatingPeriod,
		RawData:                string(respBody),
	}, nil
}
