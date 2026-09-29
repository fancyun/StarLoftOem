package upstream

import "strconv"

// TencentFaceProvider 腾讯云人脸核身 provider（账户实名备选通道）。
// 包装 TencentFaceIdClient，把腾讯云口径的 BizToken / ErrCode 归一为 provider 无关结果。
type TencentFaceProvider struct {
	client *TencentFaceIdClient
}

// NewTencentFaceProvider 创建腾讯云人脸核身 provider
func NewTencentFaceProvider(client *TencentFaceIdClient) *TencentFaceProvider {
	return &TencentFaceProvider{client: client}
}

// StartAuth 发起核身：DetectAuth 返回 H5 核身地址与 BizToken，拼接为可直接访问的核身链接
func (p *TencentFaceProvider) StartAuth(name, idCard, returnURL, bizNo string) (*FaceStartResult, error) {
	detect, err := p.client.DetectAuth(name, idCard, returnURL, bizNo)
	if err != nil {
		return nil, err
	}
	return &FaceStartResult{
		AuthURL: detect.Url + "&token=" + detect.BizToken,
		Token:   detect.BizToken,
	}, nil
}

// QueryResult 查询核身结果：ErrCode==0 通过；未返回描述视为仍在进行中
func (p *TencentFaceProvider) QueryResult(token string) (*FaceQueryResult, error) {
	r, err := p.client.GetDetectInfo(token)
	if err != nil {
		return nil, err
	}
	q := &FaceQueryResult{
		Success: r.ErrCode == 0,
		Code:    strconv.Itoa(r.ErrCode),
		Message: r.Description,
	}
	if !q.Success && r.Description == "" {
		q.Pending = true
	}
	return q, nil
}

var _ FaceProvider = (*TencentFaceProvider)(nil)