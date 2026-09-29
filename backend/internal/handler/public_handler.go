package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	qrcode "github.com/skip2/go-qrcode"

	"oemrpa/internal/config"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/runtime"
)

type PublicHandler struct {
	rt          *runtime.Runtime
	settingRepo *repository.SettingRepository
	// contactDef 启动时生效的客服联系方式（数据库配置或代码内置默认值），用于配置表无值时的兜底
	contactDef config.ContactConfig
}

func NewPublicHandler(rt *runtime.Runtime, settingRepo *repository.SettingRepository, cfg *config.Config) *PublicHandler {
	return &PublicHandler{
		rt:          rt,
		settingRepo: settingRepo,
		contactDef:  cfg.Contact,
	}
}

// GetPublicConfig 获取公开配置（无需登录）
func (h *PublicHandler) GetPublicConfig(c *gin.Context) {
	// 已启用的在线支付渠道（未配置凭据的渠道不返回，前端据此隐藏支付方式）
	channels := make([]string, 0, 2)
	if h.rt.Alipay() != nil {
		channels = append(channels, model.ChannelAlipay)
	}
	if h.rt.WechatPay() != nil {
		channels = append(channels, model.ChannelWechat)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"captcha_app_id":   h.rt.CaptchaAppID(),
			"fv_auth_price":    h.rt.FvAuthPrice(), // 下游有源人脸核验单价（前端展示用）
			"fv_self_price":    h.rt.FvSelfPrice(), // 下游无源人脸核验单价（前端展示用）
			"payment_channels": channels,           // 已启用的在线支付渠道
			"contact":          h.contact(),        // 客服联系方式（门户首页展示）
		},
	})
}

// contact 客服联系方式：实时读取系统库配置表（后台「系统设置 → 客服联系方式」维护），
// 配置行存在即以其值为准（留空表示该项不在首页展示），配置行不存在或读取失败时回落启动时生效值。
// 故后台改完门户即刻生效、无需重启后端。
func (h *PublicHandler) contact() gin.H {
	kv, err := h.settingRepo.AllSettings()
	if err != nil {
		kv = nil
	}
	pick := func(key, def string) string {
		if v, ok := kv[key]; ok {
			return strings.TrimSpace(v)
		}
		return def
	}
	return gin.H{
		"email":  pick(config.SettingKeyContactEmail, h.contactDef.Email),
		"phone":  pick(config.SettingKeyContactPhone, h.contactDef.Phone),
		"wechat": pick(config.SettingKeyContactWechat, h.contactDef.Wechat),
		"qq":     pick(config.SettingKeyContactQQ, h.contactDef.QQ),
		"hours":  pick(config.SettingKeyContactHours, h.contactDef.Hours),
	}
}

// GetQRCode 渲染二维码图片
func (h *PublicHandler) GetQRCode(c *gin.Context) {
	data := c.Query("data")
	if data == "" || len(data) > 2048 {
		c.String(http.StatusBadRequest, "invalid data")
		return
	}

	size := 280
	if s := c.Query("size"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 100 && n <= 1000 {
			size = n
		}
	}

	png, err := qrcode.Encode(data, qrcode.Medium, size)
	if err != nil {
		c.String(http.StatusInternalServerError, "qr encode failed")
		return
	}
	c.Data(http.StatusOK, "image/png", png)
}
