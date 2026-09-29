package handler

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/config"
	"oemrpa/internal/middleware"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
)

// SettingHandler 平台配置管理：系统库第三方密钥与产品库产品配置
type SettingHandler struct {
	settingRepo *repository.SettingRepository
}

func NewSettingHandler(settingRepo *repository.SettingRepository) *SettingHandler {
	return &SettingHandler{settingRepo: settingRepo}
}

// settingItem 系统配置展示项（敏感值脱敏）
type settingItem struct {
	ConfigKey   string `json:"config_key"`
	ConfigValue string `json:"config_value"`
	Category    string `json:"category"`
	Remark      string `json:"remark"`
	HasValue    bool   `json:"has_value"`
	Sensitive   bool   `json:"sensitive"`
}

// GetSettings 系统配置列表（GET /admin/settings?category=）
func (h *SettingHandler) GetSettings(c *gin.Context) {
	list, err := h.settingRepo.ListSettings(strings.TrimSpace(c.Query("category")))
	if err != nil {
		failPromotion(c, 500, "查询失败")
		return
	}

	items := make([]*settingItem, 0, len(list))
	for _, s := range list {
		it := &settingItem{ConfigKey: s.ConfigKey, Category: s.Category, Remark: s.Remark, HasValue: s.ConfigValue != ""}
		if config.IsSecretSettingKey(s.ConfigKey) {
			it.Sensitive = true
		} else {
			it.ConfigValue = s.ConfigValue
		}
		items = append(items, it)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"list": items}})
}

// UpsertSetting 新增/更新系统配置（PUT /admin/settings）
// 配置表只保存非密钥类配置；密钥类（判定见 config.IsSecretSettingKey）一律只保存在 .env，不接受写入。
func (h *SettingHandler) UpsertSetting(c *gin.Context) {
	var req struct {
		ConfigKey   string `json:"config_key" binding:"required"`
		ConfigValue string `json:"config_value"`
		Category    string `json:"category"`
		Remark      string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failPromotion(c, 400, "invalid request parameters")
		return
	}

	key := strings.TrimSpace(req.ConfigKey)
	if key == "" {
		failPromotion(c, 400, "配置键不能为空")
		return
	}
	if config.IsSecretSettingKey(key) {
		failPromotion(c, 400, "密钥类配置只保存在 .env，不支持在后台维护")
		return
	}

	category := req.Category
	if category == "" {
		category = model.SettingCategoryCommon
	}
	if err := h.settingRepo.UpsertSetting(key, req.ConfigValue, category, req.Remark); err != nil {
		failPromotion(c, 500, "保存失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// DeleteSetting 删除系统配置（DELETE /admin/settings?config_key=）
func (h *SettingHandler) DeleteSetting(c *gin.Context) {
	key := strings.TrimSpace(c.Query("config_key"))
	if key == "" {
		failPromotion(c, 400, "invalid request parameters")
		return
	}
	if err := h.settingRepo.DeleteSetting(key); err != nil {
		failPromotion(c, 500, "删除失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// GetProductConfigs 产品配置列表（GET /admin/product-config?product=fv|sms，缺省返回全部产品）。
// 按账号权限过滤：无某产品配置权限时该产品不返回（指定 product 且无权限则直接拒绝）。
func (h *SettingHandler) GetProductConfigs(c *gin.Context) {
	product := strings.TrimSpace(c.Query("product"))
	if product != "" && product != "fv" && product != "sms" {
		failPromotion(c, 400, "不支持的产品")
		return
	}
	if product != "" && !middleware.CanAccessProductConfig(c, product, false) {
		failPromotion(c, 403, "无权限访问该产品配置")
		return
	}

	data := gin.H{}
	for _, p := range []struct {
		key string
		db  string
	}{{"fv", model.FvDB}, {"sms", model.SmsDB}} {
		if product != "" && product != p.key {
			continue
		}
		if !middleware.CanAccessProductConfig(c, p.key, false) {
			continue
		}
		list, err := h.settingRepo.ListProductConfigs(p.db)
		if err != nil {
			failPromotion(c, 500, "查询失败")
			return
		}
		data[p.key] = list
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    data,
	})
}

// UpsertProductConfig 新增/更新产品配置（PUT /admin/product-config）
func (h *SettingHandler) UpsertProductConfig(c *gin.Context) {
	var req struct {
		Product     string `json:"product" binding:"required"` // fv / sms
		ConfigKey   string `json:"config_key" binding:"required"`
		ConfigValue string `json:"config_value"`
		Remark      string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		failPromotion(c, 400, "invalid request parameters")
		return
	}

	dbName := ""
	switch req.Product {
	case "fv":
		dbName = model.FvDB
	case "sms":
		dbName = model.SmsDB
	default:
		failPromotion(c, 400, "不支持的产品")
		return
	}
	if !middleware.CanAccessProductConfig(c, req.Product, true) {
		failPromotion(c, 403, "无权限修改该产品配置")
		return
	}

	if err := h.settingRepo.UpsertProductConfig(dbName, strings.TrimSpace(req.ConfigKey), req.ConfigValue, req.Remark); err != nil {
		failPromotion(c, 500, "保存失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// DeleteProductConfig 删除产品配置（DELETE /admin/product-config?product=&config_key=）
func (h *SettingHandler) DeleteProductConfig(c *gin.Context) {
	product := c.Query("product")
	dbName := ""
	switch product {
	case "fv":
		dbName = model.FvDB
	case "sms":
		dbName = model.SmsDB
	default:
		failPromotion(c, 400, "不支持的产品")
		return
	}
	if !middleware.CanAccessProductConfig(c, product, true) {
		failPromotion(c, 403, "无权限修改该产品配置")
		return
	}
	key := strings.TrimSpace(c.Query("config_key"))
	if key == "" {
		failPromotion(c, 400, "invalid request parameters")
		return
	}
	if err := h.settingRepo.DeleteProductConfig(dbName, key); err != nil {
		failPromotion(c, 500, "删除失败")
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}
