package upstream

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"oemrpa/internal/logstore"
)

const (
	wechatOAuthMPAuthorizeURL = "https://open.weixin.qq.com/connect/oauth2/authorize" // 公众号网页授权
	wechatOAuthTokenURL       = "https://api.weixin.qq.com/sns/oauth2/access_token"   // code 换 access_token/openid
)

// 微信登录场景：唯一场景为公众号网页授权（手机端在微信内置浏览器打开；
// PC 端由前端渲染该授权链接的二维码，用户用微信扫一扫在微信内置浏览器中完成授权）
const SceneMP = "mp"

// WechatOAuthUser 微信授权返回的登录身份标识
type WechatOAuthUser struct {
	OpenID     string
	UnionID    string
	Nickname   string
	HeadImgURL string
	Scene      string
}

// WechatOAuthClient 微信登录客户端（仅公众号网页授权）。
// 公众号若已绑定同一微信开放平台账号，授权响应会带回 unionid（可能为空，调用方须容错）。
type WechatOAuthClient struct {
	mpAppID     string
	mpAppSecret string
	httpClient  *http.Client
}

// NewWechatOAuthClient 创建微信登录客户端；AppID 未配置时返回 (nil, nil)。
func NewWechatOAuthClient(mpAppID, mpAppSecret string) (*WechatOAuthClient, error) {
	if mpAppID == "" {
		return nil, nil
	}
	return &WechatOAuthClient{
		mpAppID:     mpAppID,
		mpAppSecret: mpAppSecret,
		httpClient:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Available 判断公众号凭据是否齐备（AppID 与 AppSecret 均有值）
func (c *WechatOAuthClient) Available() bool {
	return c.mpAppID != "" && c.mpAppSecret != ""
}

// AuthorizeURL 拼装公众号网页授权地址（snsapi_base 静默授权）。
// redirectURI 为微信回跳地址，其域名须与微信后台登记的「网页授权域名」一致。
func (c *WechatOAuthClient) AuthorizeURL(redirectURI, state string) (string, error) {
	if !c.Available() {
		return "", errors.New("微信登录未配置")
	}
	q := url.Values{}
	q.Set("appid", c.mpAppID)
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("scope", "snsapi_base")
	q.Set("state", state)
	return wechatOAuthMPAuthorizeURL + "?" + q.Encode() + "#wechat_redirect", nil
}

// ExchangeCode 用授权 code 换取用户标识（openid 与可选 unionid）。
func (c *WechatOAuthClient) ExchangeCode(code string) (*WechatOAuthUser, error) {
	if !c.Available() {
		return nil, errors.New("微信登录未配置")
	}

	q := url.Values{}
	q.Set("appid", c.mpAppID)
	q.Set("secret", c.mpAppSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")

	body, err := c.get("/sns/oauth2/access_token", wechatOAuthTokenURL, q)
	if err != nil {
		return nil, err
	}
	var r struct {
		ErrCode int    `json:"errcode"`
		ErrMsg  string `json:"errmsg"`
		OpenID  string `json:"openid"`
		UnionID string `json:"unionid"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("解析微信授权响应失败: %w", err)
	}
	if r.ErrCode != 0 {
		return nil, fmt.Errorf("微信授权失败: errcode=%d errmsg=%s", r.ErrCode, r.ErrMsg)
	}
	if r.OpenID == "" {
		return nil, errors.New("微信授权未返回 openid")
	}
	return &WechatOAuthUser{OpenID: r.OpenID, UnionID: r.UnionID, Scene: SceneMP}, nil
}

// get 发起微信接口 GET 请求并写入 syscall.log。
// 日志只记接口路径与状态码：请求串含 AppSecret、code 等敏感值，一律不入日志。
func (c *WechatOAuthClient) get(refPath, baseURL string, q url.Values) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, baseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		logstore.RecordSysCall("request", "wechat-oauth", "", 0, refPath, "", "", 0)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	logstore.RecordSysCall("request", "wechat-oauth", "", 0, refPath, "", "", resp.StatusCode)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("微信接口请求失败: status=%d", resp.StatusCode)
	}
	return body, nil
}
