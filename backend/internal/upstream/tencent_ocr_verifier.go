package upstream

// TencentOcrVerifier 腾讯云 OCR 企业工商四要素核验 provider。
// 包装 TencentOcrClient，把腾讯云口径的结果归一为 provider 无关结果。
type TencentOcrVerifier struct {
	client *TencentOcrClient
}

// NewTencentOcrVerifier 创建腾讯云企业四要素核验 provider
func NewTencentOcrVerifier(client *TencentOcrClient) *TencentOcrVerifier {
	return &TencentOcrVerifier{client: client}
}

// Verify 核验企业名称、统一社会信用代码、法人姓名、法人身份证号四要素一致性
func (v *TencentOcrVerifier) Verify(companyName, creditCode, legalName, legalIDCard string) (*EnterpriseVerifyResult, error) {
	r, err := v.client.VerifyBizLicenseEnterprise4(companyName, creditCode, legalName, legalIDCard)
	if err != nil {
		return nil, err
	}
	res := &EnterpriseVerifyResult{
		Matched: r.VerifyResult == 1,
		RawData: r.RawData,
	}
	if !res.Matched {
		res.Message = "企业信息核验未通过"
	}
	return res, nil
}

var _ EnterpriseVerifier = (*TencentOcrVerifier)(nil)