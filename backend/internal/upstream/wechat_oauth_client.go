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
	wechatOAuthMPAuthorizeURL = "https://open.weixin.qq.com/connect/oauth2/authorize" // 公众号网页授权（手机端）
	wechatOAuthQRConnectURL   = "https://open.weixin.qq.com/connect/qrconnect"        // 开放平台网站应用扫码（PC）
	wechatOAuthTokenURL       = "https://api.weixin.qq.com/sns/oauth2/access_token"   // code 换 access_token/openid
	wechatOAuthUserInfoURL    = "https://api.weixin.qq.com/sns/userinfo"              // 拉取用户昵称/头像（仅扫码端）
)

// 微信登录场景：mp-公众号网页授权（手机端） pc-开放平台网站应用扫码（PC 端）
const (
	SceneMP = "mp"
	ScenePC = "pc"
)

// WechatOAuthUser 微信授权返回的登录身份标识
type WechatOAuthUser struct {
	OpenID     string
	UnionID    string
	Nickname   string
	HeadImgURL string
	Scene      string
}

// WechatOAuthClient 微信登录客户端（公众号网页授权 + 开放平台网站应用扫码）。
// 两套应用各自持有 AppID/AppSecret：公众号 openid 与网站应用 openid 不同，
// unionid 仅在两者绑定到同一微信开放平台账号时返回（可能为空，调用方须容错）。
type WechatOAuthClient struct {
	mpAppID       string
	mpAppSecret   string
	openAppID     string
	openAppSecret string
	httpClient    *http.Client
}

// NewWechatOAuthClient 创建微信登录客户端；两套凭据均未配置时返回 (nil, nil)。
func NewWechatOAuthClient(mpAppID, mpAppSecret, openAppID, openAppSecret string) (*WechatOAuthClient, error) {
	if mpAppID == "" && openAppID == "" {
		return nil, nil
	}
	return &WechatOAuthClient{
		mpAppID:       mpAppID,
		mpAppSecret:   mpAppSecret,
		openAppID:     openAppID,
		openAppSecret: openAppSecret,
		httpClient:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Available 判断指定场景的凭据是否齐备（AppID 与 AppSecret 均有值）
func (c *WechatOAuthClient) Available(scene string) bool {
	switch scene {
	case SceneMP:
		return c.mpAppID != "" && c.mpAppSecret != ""
	case ScenePC:
		return c.openAppID != "" && c.openAppSecret != ""
	}
	return false
}

// AuthorizeURL 拼装微信授权地址：mp 走公众号网页授权（snsapi_base 静默授权），pc 走开放平台扫码（snsapi_login）。
// redirectURI 为微信回跳地址，其域名须与微信后台登记的「网页授权域名 / 授权回调域」一致。
func (c *WechatOAuthClient) AuthorizeURL(scene, redirectURI, state string) (string, error) {
	if !c.Available(scene) {
		return "", errors.New("该场景微信登录未配置")
	}
	q := url.Values{}
	q.Set("redirect_uri", redirectURI)
	q.Set("response_type", "code")
	q.Set("state", state)
	base := wechatOAuthMPAuthorizeURL
	if scene == ScenePC {
		base = wechatOAuthQRConnectURL
		q.Set("appid", c.openAppID)
		q.Set("scope", "snsapi_login")
	} else {
		q.Set("appid", c.mpAppID)
		q.Set("scope", "snsapi_base")
	}
	return base + "?" + q.Encode() + "#wechat_redirect", nil
}

// ExchangeCode 用授权 code 换取用户标识；pc 场景额外拉取昵称与头像（snsapi_base 无该权限）。
func (c *WechatOAuthClient) ExchangeCode(scene, code string) (*WechatOAuthUser, error) {
	if !c.Available(scene) {
		return nil, errors.New("该场景微信登录未配置")
	}
	appID, appSecret := c.mpAppID, c.mpAppSecret
	if scene == ScenePC {
		appID, appSecret = c.openAppID, c.openAppSecret
	}

	q := url.Values{}
	q.Set("appid", appID)
	q.Set("secret", appSecret)
	q.Set("code", code)
	q.Set("grant_type", "authorization_code")

	body, err := c.get("/sns/oauth2/access_token", wechatOAuthTokenURL, q)
	if err != nil {
		return nil, err
	}
	var r struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		OpenID      string `json:"openid"`
		UnionID     string `json:"unionid"`
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

	u := &WechatOAuthUser{OpenID: r.OpenID, UnionID: r.UnionID, Scene: scene}
	// 扫码端可拉取用户信息（失败不影响登录，昵称仅用于展示）
	if scene == ScenePC && r.AccessToken != "" {
		if nickname, headImg, ierr := c.fetchUserInfo(r.AccessToken, r.OpenID); ierr == nil {
			u.Nickname, u.HeadImgURL = nickname, headImg
		}
	}
	return u, nil
}

// fetchUserInfo 拉取微信用户昵称与头像（仅开放平台扫码端有权限）
func (c *WechatOAuthClient) fetchUserInfo(accessToken, openID string) (nickname, headImgURL string, err error) {
	q := url.Values{}
	q.Set("access_token", accessToken)
	q.Set("openid", openID)
	q.Set("lang", "zh_CN")

	body, err := c.get("/sns/userinfo", wechatOAuthUserInfoURL, q)
	if err != nil {
		return "", "", err
	}
	var r struct {
		ErrCode    int    `json:"errcode"`
		ErrMsg     string `json:"errmsg"`
		Nickname   string `json:"nickname"`
		HeadImgURL string `json:"headimgurl"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return "", "", fmt.Errorf("解析微信用户信息失败: %w", err)
	}
	if r.ErrCode != 0 {
		return "", "", fmt.Errorf("微信用户信息拉取失败: errcode=%d errmsg=%s", r.ErrCode, r.ErrMsg)
	}
	return r.Nickname, r.HeadImgURL, nil
}

// get 发起微信接口 GET 请求并写入 syscall.log。
// 日志只记接口路径与状态码：请求串含 AppSecret、code、access_token 等敏感值，一律不入日志。
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