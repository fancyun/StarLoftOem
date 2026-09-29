package starloft

import (
	"fmt"
	"net/http"
	"strings"

	"oemrpa/internal/model"
	"oemrpa/internal/upstream"
)

// 人脸核身发起地址（账户实名固定走有源核身）
const accountFacePath = "/v1/fv/auth"

// StartAuth 发起账户实名人脸核身：POST /v1/fv/auth。
// 平台返回自站承接链接，本平台直接交给浏览器打开完成核身；
// 返回的 Token 为平台业务号，用于后续 QueryResult 查询核身结果。
func (c *Client) StartAuth(name, idCard, returnURL, bizNo string) (*upstream.FaceStartResult, error) {
	payload := map[string]interface{}{
		"name":           name,
		"id_card":        idCard,
		"biz_extra_data": bizNo, // 本平台实名记录单号随之上送，便于对账
	}
	if returnURL != "" {
		payload["return_url"] = returnURL
	}

	var out struct {
		BizNo       string `json:"biz_no"`
		SiteURL     string `json:"site_url"`
		ExpiredTime int64  `json:"expired_time"`
	}
	if err := c.call(http.MethodPost, accountFacePath, "kyc-auth", bizNo, payload, &out); err != nil {
		return nil, err
	}
	if out.SiteURL == "" || out.BizNo == "" {
		return nil, fmt.Errorf("StarLoft 平台未返回核身地址或业务号")
	}
	return &upstream.FaceStartResult{AuthURL: out.SiteURL, Token: out.BizNo, ExpireAt: out.ExpiredTime}, nil
}

// QueryResult 查询账户实名核身结果：POST /v1/fv/result。
// 平台认证状态（0-待认证 1-认证中 2-成功 3-失败 4-已取消 5-超时 6-发起失败）归一为
// 通过 / 进行中 / 终态失败 三种结论。
func (c *Client) QueryResult(token string) (*upstream.FaceQueryResult, error) {
	if strings.TrimSpace(token) == "" {
		return nil, fmt.Errorf("业务号不能为空")
	}
	var out struct {
		BizNo         string `json:"biz_no"`
		Status        int    `json:"status"`
		ResultCode    string `json:"result_code"`
		ResultMessage string `json:"result_message"`
	}
	if err := c.call(http.MethodPost, "/v1/fv/result", "kyc-result", token,
		map[string]interface{}{"biz_no": token}, &out); err != nil {
		return nil, err
	}

	q := &upstream.FaceQueryResult{Code: out.ResultCode, Message: out.ResultMessage}
	switch out.Status {
	case model.AuthStatusSuccess:
		q.Success = true
	case model.AuthStatusPending, model.AuthStatusProcessing:
		q.Pending = true
	default:
		if q.Message == "" {
			q.Message = "认证未通过"
		}
	}
	return q, nil
}

var _ upstream.FaceProvider = (*Client)(nil)