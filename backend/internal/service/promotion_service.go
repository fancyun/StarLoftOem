package service

import (
	"crypto/rand"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"oemrpa/internal/audit"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/site"
)

// billableServices 支持用户级定价的服务标识
var billableServices = []string{
	model.ServiceFVAuth,
	model.ServiceFVSelf,
	model.ServiceKYCPersonal,
	model.ServiceKYCEnterprise,
	model.ServiceSMS,
}

// PromotionService 推广分佣：推广码、归属解析、价格解析、计费调度与提成计提。
//
// 推广身份不再有独立主体表：推广码落在 user.aff_code / admin_user.aff_code，
// 用户归属落在 user.referrer_type + referrer_id，提成按 referrer 归属分表：
//   - 用户型推广（user）：落 user_commission，仅归因被推广用户注册后 1 个自然月内的消费；
//   - 员工销售（staff）：落 staff_commission，长期有效，不参与提现。
type PromotionService struct {
	promotionRepo *repository.PromotionRepository
	userRepo      *repository.UserRepository
	adminRepo     *repository.AdminRepository
	balance       *BalanceService
	// platformPrice 平台价（产品库配置，无则代码内置默认值），入参为服务标识，返回元/单位
	platformPrice func(service string) float64
	// productCost 平台成本单价（产品库配置，无则代码内置默认值），入参为服务标识，返回元/单位；
	// 仅用于「利润 = 实付 − 成本 × 件数」并按利润计提提成，未注入或为 0 时视为无成本
	productCost func(service string) float64
	// productRateFn 全局提成规则（后台系统设置可改），入参为产品档位，对所有推广方统一生效
	productRateFn func(product string) float64
	// finance 推广收益与提现服务
	finance *PromotionFinanceService
}

// SetProductRateProvider 注入全局提成比例取数函数（后台系统设置可改，按产品档位区分）
func (s *PromotionService) SetProductRateProvider(fn func(product string) float64) {
	s.productRateFn = fn
}

// ProductRate 取指定产品档位的全局提成比例（0 表示该产品不提成）：
// 由平台在「系统设置 → 推广分佣」统一配置，改完即时生效，不按推广方单独区分。
func (s *PromotionService) ProductRate(product string) float64 {
	if s.productRateFn == nil {
		return model.DefaultCommissionRate
	}
	rate := s.productRateFn(product)
	if rate < 0 || rate > 1 {
		return model.DefaultCommissionRate
	}
	return rate
}

func NewPromotionService(
	promotionRepo *repository.PromotionRepository,
	userRepo *repository.UserRepository,
	adminRepo *repository.AdminRepository,
	balance *BalanceService,
	platformPrice func(service string) float64,
	productCost func(service string) float64,
) *PromotionService {
	return &PromotionService{promotionRepo: promotionRepo, userRepo: userRepo, adminRepo: adminRepo, balance: balance, platformPrice: platformPrice, productCost: productCost}
}

// SetFinanceService 注入推广收益服务（在路由装配时调用）
func (s *PromotionService) SetFinanceService(f *PromotionFinanceService) {
	s.finance = f
}

// ---------- 归属 ----------

// ReferrerOf 解析用户的有效归属推介方：referrer_type='user' 恒有效，
// 'staff' 须其后台账号处于启用状态（停用即不再归因）；未登记或无效时 ok=false（平台直营）。
func (s *PromotionService) ReferrerOf(userID int64) (string, int64, bool) {
	referrerType, referrerID, err := s.userRepo.GetUserReferrer(userID)
	if err != nil || referrerID <= 0 {
		return "", 0, false
	}
	switch referrerType {
	case model.RefTypeReferrerUser:
		return referrerType, referrerID, true
	case model.RefTypeReferrerStaff:
		if s.adminRepo != nil {
			a, err := s.adminRepo.GetAdminByID(referrerID)
			if err != nil || a == nil || a.Status != 1 {
				return "", 0, false
			}
		}
		return referrerType, referrerID, true
	}
	return "", 0, false
}

// BillingReferrerID 返回用户有效归属的推介方编号（0 表示平台直营/无有效归属）：
// referrer_type='user' 时为 user.id，'staff' 时为 admin_user.id（须启用）。仅供调用方判定「是否计入提成」。
func (s *PromotionService) BillingReferrerID(userID int64) int64 {
	if _, referrerID, ok := s.ReferrerOf(userID); ok {
		return referrerID
	}
	return 0
}

// SiteHosts 返回用户适用的站点域名组：白标已取消，一律回落平台默认站点。
func (s *PromotionService) SiteHosts(userID int64) site.Hosts {
	return site.Platform()
}

// ---------- 推广码 ----------

// affCodeAlphabet 推广码字符集：数字 + 小写字母（36 个）
const affCodeAlphabet = "abcdefghijklmnopqrstuvwxyz0123456789"

// affCodeLength 推广码长度（12 位）
const affCodeLength = 12

// newAffCode 生成 12 位推广码（数字 + 小写字母），熵源为 crypto/rand
func newAffCode() string {
	buf := make([]byte, affCodeLength)
	if _, err := rand.Read(buf); err != nil {
		// 熵源异常时退化为纳秒时间戳派生，保证流程不中断
		return fmt.Sprintf("%012d", time.Now().UnixNano()%1000000000000)
	}
	out := make([]byte, affCodeLength)
	for i, b := range buf {
		out[i] = affCodeAlphabet[int(b)%len(affCodeAlphabet)]
	}
	return string(out)
}

// allocateAffCode 生成未被占用的推广码（最多尝试 6 次，冲突概率可忽略）。
// 推广码在 user 与 admin_user 两张表共享同一编号空间，须双表查重。
func (s *PromotionService) allocateAffCode() (string, error) {
	for i := 0; i < 6; i++ {
		code := newAffCode()
		used, err := s.userRepo.AffCodeExists(code)
		if err != nil {
			return "", err
		}
		if used {
			continue
		}
		if s.adminRepo != nil {
			if used, err = s.adminRepo.AffCodeExists(code); err != nil {
				return "", err
			}
			if used {
				continue
			}
		}
		return code, nil
	}
	return "", fmt.Errorf("生成推广码失败，请重试")
}

// EnsureUserAffCode 确保用户拥有推广码（幂等）：无码则生成并落 user.aff_code，返回推广码。
func (s *PromotionService) EnsureUserAffCode(userID int64) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("用户 ID 无效")
	}
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return "", err
	}
	if user.AffCode.Valid && strings.TrimSpace(user.AffCode.String) != "" {
		return user.AffCode.String, nil
	}
	code, err := s.allocateAffCode()
	if err != nil {
		return "", err
	}
	if err := s.userRepo.SetUserAffCode(userID, code); err != nil {
		return "", err
	}
	audit.Log("user_aff_code_create", audit.KV("user_id", userID), audit.KV("aff_code", code))
	return code, nil
}

// ResolveAffCode 按推广码解析推介方（注册 ?ref= 绑定归属用）：
// 先查用户型推广码，再查员工推广码（须员工账号启用）；未命中返回 ok=false（平台直营）。
func (s *PromotionService) ResolveAffCode(code string) (string, int64, bool) {
	// 推广码恒为小写，用户手工输入时统一转换，避免大小写不匹配查不到
	code = strings.ToLower(strings.TrimSpace(code))
	if code == "" {
		return "", 0, false
	}
	if u, err := s.userRepo.GetUserByAffCode(code); err == nil && u != nil {
		return model.RefTypeReferrerUser, u.ID, true
	}
	if s.adminRepo != nil {
		if a, err := s.adminRepo.GetAdminByAffCode(code); err == nil && a != nil && a.Status == 1 {
			return model.RefTypeReferrerStaff, a.ID, true
		}
	}
	return "", 0, false
}

// EnsureStaffAffCode 确保员工拥有销售推广码（幂等）：无码则生成并落 admin_user.aff_code，返回推广码。
func (s *PromotionService) EnsureStaffAffCode(adminID int64) (string, error) {
	if adminID <= 0 {
		return "", fmt.Errorf("员工 ID 无效")
	}
	if s.adminRepo == nil {
		return "", fmt.Errorf("员工仓储未装配")
	}
	a, err := s.adminRepo.GetAdminByID(adminID)
	if err != nil || a == nil {
		return "", fmt.Errorf("员工不存在")
	}
	if a.AffCode.Valid && strings.TrimSpace(a.AffCode.String) != "" {
		return a.AffCode.String, nil
	}
	code, err := s.allocateAffCode()
	if err != nil {
		return "", err
	}
	if err := s.adminRepo.SetAdminAffCode(adminID, code); err != nil {
		return "", err
	}
	audit.Log("staff_aff_code_create", audit.KV("admin_id", adminID), audit.KV("aff_code", code))
	return code, nil
}

// ---------- 价格解析 ----------

// UnitPrice 解析用户适用单价：用户级定向覆盖 → 平台价。
// 推广方不再具备自主定价能力，故不再沿归属链回溯覆盖价；覆盖价按服务所在产品库分表存放。
func (s *PromotionService) UnitPrice(userID int64, service string) float64 {
	table := model.PriceOverrideTableByService(service)
	if p, ok := s.overridePrice(table, model.PriceScopeUser, userID, model.PriceTypeUnit, service); ok {
		return p
	}
	return s.platformUnitPrice(service)
}

// PackPrice 解析用户适用的资源包售价：用户级定向覆盖 → 资源包原价。
// target 带产品前缀（如 fv_auth:3 / sms:3）：人脸核验与短信的资源包分属两库、主键会重复，必须按产品区分。
func (s *PromotionService) PackPrice(userID int64, product string, packID int64, defaultPrice float64) float64 {
	product = normalizePackProduct(product)
	table := model.PriceOverrideTableByService(product)
	if p, ok := s.overridePrice(table, model.PriceScopeUser, userID, model.PriceTypePack, packPriceTarget(product, packID)); ok {
		return p
	}
	return defaultPrice
}

// normalizePackProduct 资源包产品标识归一：空值按短信处理（早期数据无产品标记）
func normalizePackProduct(product string) string {
	if product == "" {
		return model.ServiceSMS
	}
	return product
}

// packPriceTarget 资源包定价标识：产品标识 + 主键
func packPriceTarget(product string, packID int64) string {
	return normalizePackProduct(product) + ":" + strconv.FormatInt(packID, 10)
}

// UserScopedUnitPrice 平台侧单价：仅应用「平台给单个用户定向定价」
func (s *PromotionService) UserScopedUnitPrice(userID int64, service string) float64 {
	table := model.PriceOverrideTableByService(service)
	if p, ok := s.overridePrice(table, model.PriceScopeUser, userID, model.PriceTypeUnit, service); ok {
		return p
	}
	return s.platformUnitPrice(service)
}

// UnitPrices 返回用户适用的全部服务单价（供前端展示）
func (s *PromotionService) UnitPrices(userID int64) map[string]float64 {
	out := make(map[string]float64, len(billableServices))
	for _, svc := range billableServices {
		out[svc] = s.UnitPrice(userID, svc)
	}
	return out
}

// PlatformPrices 返回平台基准价（供前端展示对比）
func (s *PromotionService) PlatformPrices() map[string]float64 {
	out := make(map[string]float64, len(billableServices))
	for _, svc := range billableServices {
		out[svc] = s.platformUnitPrice(svc)
	}
	return out
}

// platformUnitPrice 平台价（无配置时兜底，避免出现 0 价）
func (s *PromotionService) platformUnitPrice(service string) float64 {
	if s.platformPrice == nil {
		return 0
	}
	return s.platformPrice(service)
}

// productUnitCost 平台成本单价（元/单位）：未注入或未配置时返回 0（视为无成本，利润即实付额）
func (s *PromotionService) productUnitCost(service string) float64 {
	if s.productCost == nil {
		return 0
	}
	if c := s.productCost(service); c > 0 {
		return c
	}
	return 0
}

// profitOf 提成计提基数：利润 = 本次实付 − 单位成本 × 件数；不是正数则返回 0（无利润不提成）
func (s *PromotionService) profitOf(service string, charged float64, count int64) float64 {
	if count <= 0 {
		count = 1
	}
	if cost := s.productUnitCost(service); cost > 0 {
		charged -= cost * float64(count)
	}
	if charged <= 0 {
		return 0
	}
	return roundAmount(charged)
}

// overridePrice 查询单条价格覆盖（table 为覆盖价所在产品库的 price_override 表）
func (s *PromotionService) overridePrice(table, scopeType string, scopeID int64, priceType, target string) (float64, bool) {
	if scopeID <= 0 {
		return 0, false
	}
	p, ok, err := s.promotionRepo.GetPriceOverride(table, scopeType, scopeID, priceType, target)
	if err != nil {
		log.Printf("查询价格覆盖失败 [%s/%s/%d/%s/%s]: %v", table, scopeType, scopeID, priceType, target, err)
		return 0, false
	}
	return p, ok
}

// ---------- 定向定价配置（仅平台对单个用户定价；覆盖价按产品分库） ----------

// UnitPriceItem 单用户定向定价行：服务标识、平台价、当前生效价与用户级覆盖价（无覆盖为 nil）
type UnitPriceItem struct {
	Target         string   `json:"target"`
	PlatformPrice  float64  `json:"platform_price"`
	EffectivePrice float64  `json:"effective_price"`
	CustomPrice    *float64 `json:"custom_price"`
}

// unitPriceTargets 服务标识白名单（限定在给定服务集合内，越界直接报错）
func unitPriceTargets(services []string) map[string]bool {
	allow := make(map[string]bool, len(services))
	for _, svc := range services {
		allow[svc] = true
	}
	return allow
}

// UnitPriceOverview 组装指定服务集合的用户定向定价视图（含平台价与当前生效价）。
// 覆盖价按服务所在产品库分表读取，同一张表只查一次。
func (s *PromotionService) UnitPriceOverview(userID int64, services []string) ([]*UnitPriceItem, error) {
	allow := unitPriceTargets(services)
	custom := make(map[string]float64, len(services)) // 已设置的覆盖价
	if userID > 0 {
		tables := make(map[string]bool, len(services))
		for _, svc := range services {
			tables[model.PriceOverrideTableByService(svc)] = true
		}
		for table := range tables {
			items, err := s.promotionRepo.ListPriceOverrides(table, model.PriceScopeUser, userID)
			if err != nil {
				return nil, err
			}
			for _, it := range items {
				if it.PriceType != model.PriceTypeUnit || !allow[it.Target] {
					continue
				}
				custom[it.Target] = it.Price
			}
		}
	}

	out := make([]*UnitPriceItem, 0, len(services))
	for _, svc := range services {
		item := &UnitPriceItem{
			Target:         svc,
			PlatformPrice:  s.platformUnitPrice(svc),
			EffectivePrice: s.UnitPrice(userID, svc),
		}
		if p, ok := custom[svc]; ok {
			price := p
			item.CustomPrice = &price
		}
		out = append(out, item)
	}
	return out, nil
}

// SaveUnitPriceOverrides 保存用户在指定服务集合内的按次定价：items 批量写入，deleted 逐项删除（回落平台价）。
// 服务标识不在集合内时直接报错，避免越权改动其它产品/服务的定价。
func (s *PromotionService) SaveUnitPriceOverrides(userID int64, services []string, items, deleted []*model.PriceOverride) error {
	allow := unitPriceTargets(services)
	write := func(table, target string, price float64) error {
		if !allow[target] {
			return fmt.Errorf("不允许对该服务定价: %s", target)
		}
		return s.promotionRepo.UpsertPriceOverride(table, model.PriceScopeUser, userID, model.PriceTypeUnit, target, price)
	}
	for _, it := range items {
		if it.Target == "" || it.Price < 0 {
			continue
		}
		if err := write(model.PriceOverrideTableByService(it.Target), it.Target, it.Price); err != nil {
			return err
		}
	}
	for _, it := range deleted {
		if it.Target == "" {
			continue
		}
		if !allow[it.Target] {
			return fmt.Errorf("不允许对该服务定价: %s", it.Target)
		}
		if err := s.promotionRepo.DeletePriceOverride(model.PriceOverrideTableByService(it.Target), model.PriceScopeUser, userID, model.PriceTypeUnit, it.Target); err != nil {
			return err
		}
	}
	return nil
}

// SearchUsers 按手机号/用户名检索用户（定向定价选人用，仅返回 id + 手机号 + 用户名）
func (s *PromotionService) SearchUsers(keyword string, limit int) ([]*UserBrief, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	users, _, err := s.userRepo.GetAllUsers(1, limit, keyword, nil, "", "")
	if err != nil {
		return nil, err
	}
	out := make([]*UserBrief, 0, len(users))
	for _, u := range users {
		out = append(out, &UserBrief{ID: u.ID, Phone: u.Phone, Username: u.Username})
	}
	return out, nil
}

// UserBrief 用户简要信息（后台选人下拉用）
type UserBrief struct {
	ID       int64  `json:"id"`
	Phone    string `json:"phone"`
	Username string `json:"username"`
}

// ---------- 概览与流水 ----------

// PromotionProfile 推广控制台概览（用户型推广）
type PromotionProfile struct {
	AffCode        string             `json:"aff_code"`
	PromoLink      string             `json:"promo_link"`
	SubUserCount   int64              `json:"sub_user_count"`
	Balance        float64            `json:"balance"` // 平台余额（用户侧消费扣费来源）
	UnitPrices     map[string]float64 `json:"unit_prices"`
	PlatformPrices map[string]float64 `json:"platform_prices"`
	Finance        *AffFinanceView    `json:"finance"` // 提成收益（累计 / 已提现 / 可提现）
}

// Profile 组装用户控制台的推广概览（推广码缺省时自动生成，推广页打开即开通）
func (s *PromotionService) Profile(userID int64) (*PromotionProfile, error) {
	code, err := s.EnsureUserAffCode(userID)
	if err != nil {
		return nil, err
	}
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	p := &PromotionProfile{
		AffCode:        code,
		PromoLink:      site.Platform().ConsoleBase() + "/register?ref=" + code,
		Balance:        user.Balance,
		UnitPrices:     s.UnitPrices(userID),
		PlatformPrices: s.PlatformPrices(),
	}
	_, total, err := s.ListSubUsers(model.RefTypeReferrerUser, userID, 1, 1)
	if err != nil {
		return nil, err
	}
	p.SubUserCount = total
	if s.finance != nil {
		if p.Finance, err = s.finance.CommissionView(model.RefTypeReferrerUser, userID); err != nil {
			return nil, err
		}
	}
	return p, nil
}

// ListSubUsers 分页查询归属该推广方的下级用户（白名单字段，避免泄露证件号/余额等敏感信息）
func (s *PromotionService) ListSubUsers(referrerType string, referrerID int64, page, pageSize int) ([]*repository.AffSubUser, int64, error) {
	return s.userRepo.ListAffSubUsers(referrerType, referrerID, page, pageSize)
}

// ListSubUsersByRange 分页查询下级用户，可按注册时间区间过滤（销售报告按月查看）
func (s *PromotionService) ListSubUsersByRange(referrerType string, referrerID int64, startDate, endDate string, page, pageSize int) ([]*repository.AffSubUser, int64, error) {
	return s.userRepo.ListAffSubUsersByRange(referrerType, referrerID, startDate, endDate, page, pageSize)
}

// ListSettlements 分页查询用户型推广的提成流水
func (s *PromotionService) ListSettlements(referrerType string, referrerID int64, page, pageSize int) ([]*model.UserCommission, int64, error) {
	return s.promotionRepo.ListSettlements(referrerType, referrerID, page, pageSize)
}

// ListStaffCommissions 分页查询员工销售的提成流水（不限时间）
func (s *PromotionService) ListStaffCommissions(referrerType string, referrerID int64, page, pageSize int) ([]*model.UserCommission, int64, error) {
	return s.promotionRepo.ListStaffCommissions(referrerType, referrerID, page, pageSize)
}

// ListStaffCommissionsByRange 分页查询员工销售的提成流水，可按时间区间过滤（销售报告按月查看）
func (s *PromotionService) ListStaffCommissionsByRange(referrerType string, referrerID int64, startDate, endDate string, page, pageSize int) ([]*model.UserCommission, int64, error) {
	return s.promotionRepo.ListStaffCommissionsByRange(referrerType, referrerID, startDate, endDate, page, pageSize)
}

// ListAllCommissions 平台后台：分页查询全部提成记录（user_commission 与 staff_commission 两表合并）
func (s *PromotionService) ListAllCommissions(referrerType, bizType, keyword, startDate, endDate string, page, pageSize int) ([]*repository.CommissionRecord, int64, error) {
	return s.promotionRepo.ListAllCommissions(referrerType, bizType, keyword, startDate, endDate, page, pageSize)
}

// ---------- 资源包购买 ----------

// PurchasePack 购买人脸核验类资源包：统一走平台余额购买。
// 提成由余额服务在成交后按利润计提，且已覆盖「余额全额支付」与「余额 + 在线组合支付」两条路径，
// 此处不再计提，避免同一笔购包写两条提成流水（历史遗留的重复计提）。
func (s *PromotionService) PurchasePack(userID, packID int64) (*model.UserResourcePack, error) {
	return s.balance.PurchaseResourcePack(userID, packID)
}

// PurchaseSmsPack 购买短信资源包：短信资源包是短信库独立表，余额服务不负责其提成，故在此按利润计提。
func (s *PromotionService) PurchaseSmsPack(userID, packID int64) (*model.SmsUserResourcePack, error) {
	price, count := 0.0, int64(0)
	if pack, err := s.balance.smsResourcePackRepo.GetPackByID(packID); err == nil && pack != nil {
		price = s.PackPrice(userID, pack.Product, packID, pack.Price)
		count = int64(pack.TotalCount)
	}
	up, err := s.balance.PurchaseSmsResourcePack(userID, packID)
	if err != nil {
		return nil, err
	}
	s.accrueCommission(userID, model.ServiceSMS, price, count, model.CommissionBizPackPurchase, "sms_user_resource_pack", packID, "购买短信资源包提成")
	return up, nil
}

// ---------- 计费 ----------

// ChargeUnitFee 计费：按用户适用单价计算应付金额，统一走平台扣费（资源包优先、余额兜底），
// 余额实付部分按下级成交额计提推广提成。
// service 决定适用单价/成本/提成档位；packProduct 决定扣哪一类资源包（短信按验证码/通知与营销分开匹配）。
// 返回实际计费方式（model.PayTypePack/PayTypeBalance）、命中的资源包 ID（余额计费为 0）
// 与本次实际金额（资源包计费为 0）。
func (s *PromotionService) ChargeUnitFee(userID int64, service, packProduct string, count int64, orderID int64, remark, refType, bizType string) (int, int64, float64, error) {
	if count <= 0 {
		count = 1
	}
	amount := roundAmount(s.UnitPrice(userID, service) * float64(count))

	payType, packID, charged, err := s.chargeDirect(userID, service, packProduct, count, amount, orderID, remark, refType)
	if err != nil {
		return 0, 0, 0, err
	}
	// 资源包计费已在购包时计提提成，此处仅对余额实付部分按利润计提
	s.accrueCommission(userID, service, charged, count, bizType, refType, orderID, remark)
	return payType, packID, charged, nil
}

// RefundUnitFee 退还计费：按平台语义原路退还（资源包退次数、余额退余额），并冲回此前计提的提成。
func (s *PromotionService) RefundUnitFee(userID int64, service string, payType int, packID int64, amount float64, count int64, orderID int64, remark, refType string) error {
	if err := s.balance.RefundProductFee(userID, amount, int64(payType), packID, orderID, remark, refType); err != nil {
		return err
	}
	s.refundCommission(userID, amount, model.CommissionBizRefund, refType, orderID, remark)
	return nil
}

// chargeDirect 平台扣费（资源包优先、余额兜底）
func (s *PromotionService) chargeDirect(userID int64, service, packProduct string, count int64, amount float64, orderID int64, remark, refType string) (int, int64, float64, error) {
	if service == model.ServiceSMS {
		payType, packID, err := s.balance.DeductSmsFee(userID, amount, count, orderID, remark, refType, packProduct)
		if err != nil {
			return 0, 0, 0, err
		}
		return payType, packID, chargedAmount(payType, amount), nil
	}
	// 资源包按子产品隔离匹配：须与发起前的额度预检使用同一标识（fv_auth/fv_self），
	// 不能用一级产品（fv）归一代子产品，否则预检命中资源包、实扣却匹配不到而回落余额
	payType, packID, err := s.balance.DeductProductFee(userID, service, service, amount, orderID, remark, refType)
	if err != nil {
		return 0, 0, 0, err
	}
	return payType, packID, chargedAmount(payType, amount), nil
}

// ---------- 提成计提 ----------

// accrueCommission 按「利润」计提推广提成（失败仅记日志，不影响主流程）。
// 计提基数 = 利润 = 本次实付 charged − 单位成本 × count（成本取产品配置，件数由调用方给出）；
// 利润不是正数时不计提（成本 ≥ 售价即为无利润，不产生负提成）。
// bizType 取值见 model.CommissionBiz*；提成比例按成交所属产品档位取（人脸核验 / 短信分别配置，全局统一）。
//
// 用户型推广（user）另设归因窗口：仅被推广用户自注册起 1 个自然月内的消费计提（见 withinAffRewardWindow）；
// 员工销售（staff）不受窗口限制，长期有效。
func (s *PromotionService) accrueCommission(userID int64, service string, charged float64, count int64, bizType, refType string, refID int64, remark string) {
	if charged <= 0 {
		return
	}
	profit := s.profitOf(service, charged, count)
	if profit <= 0 {
		return
	}
	product := model.CommissionProductFor(bizType, refType)
	referrerType, referrerID, rate, ok := s.commissionTarget(userID, product)
	if !ok {
		return
	}
	if referrerType == model.RefTypeReferrerUser && !s.withinAffRewardWindow(userID) {
		log.Printf("超出 aff 奖励归因窗口（注册后 %d 个自然月），跳过计提 [user=%d, referrer=%d, %s#%d]",
			model.AffRewardWindowMonths, userID, referrerID, refType, refID)
		return
	}
	s.writeCommission(referrerType, referrerID, userID, roundAmount(profit*rate), bizType, refType, refID, remark)
}

// withinAffRewardWindow 判断被推广用户是否仍在用户型推广的提成归因窗口内（自注册起 1 个自然月内）。
// 取不到注册时间时按「超期」处理：宁可少计提，也不在时间口径不明时误发提成。
func (s *PromotionService) withinAffRewardWindow(userID int64) bool {
	createdAt, err := s.userRepo.GetUserCreatedAt(userID)
	if err != nil {
		log.Printf("查询用户注册时间失败，按超出归因窗口处理 [user=%d]: %v", userID, err)
		return false
	}
	return !time.Now().After(createdAt.AddDate(0, model.AffRewardWindowMonths, 0))
}

// AccruePackCommission 资源包成交后按利润计提推广提成（供余额购买与在线支付落地路径调用）。
// service 为资源包所属服务标识（fv_auth / fv_self / sms），count 为包内次数/条数。
func (s *PromotionService) AccruePackCommission(userID int64, service string, amount float64, count int64, refType string, refID int64, remark string) {
	s.accrueCommission(userID, service, amount, count, model.CommissionBizPackPurchase, refType, refID, remark)
}

// refundCommission 冲回此前计提的推广提成（写负额流水）。
//
// 按「原流水实际计提额」冲回，而非按当前提成比例重算：提成比例可被后台事后调整，
// 若按新比例重算会出现冲回金额与实际计提不符（少冲则推广方白拿，多冲则推广方倒亏）。
// 冲回不受 aff 归因窗口影响：窗口内已计提的提成，事后退款必须原路冲回。
func (s *PromotionService) refundCommission(userID int64, amount float64, bizType, refType string, refID int64, remark string) {
	if amount <= 0 {
		return
	}
	referrerType, referrerID, ok := s.ReferrerOf(userID)
	if !ok {
		return
	}
	accrued, err := s.promotionRepo.SumCommissionByRef(model.SettlementTable(referrerType), userID, refType, refID)
	if err != nil {
		log.Printf("查询原提成流水失败，跳过冲回 [user=%d, %s#%d]: %v", userID, refType, refID, err)
		return
	}
	if accrued <= 0 {
		// 该单据已无未冲回的提成（未曾计提，或此前已全额冲回），无需再冲
		return
	}
	s.writeCommission(referrerType, referrerID, userID, -accrued, bizType, refType, refID, remark)
}

// commissionTarget 解析用户在该产品档位下的提成归属与比例（无归属、该档位比例为 0 或疑似自我推广时 ok=false）
func (s *PromotionService) commissionTarget(userID int64, product string) (string, int64, float64, bool) {
	referrerType, referrerID, ok := s.ReferrerOf(userID)
	if !ok {
		return "", 0, 0, false
	}
	// 自我推广套利校验仅适用于用户型推广（可比对手机号/实名主体）；
	// 员工型的 id 与 user.id 不在同一编号空间，无从比对，且提成基数为利润，自购无净收益。
	if referrerType == model.RefTypeReferrerUser {
		self, err := s.userRepo.IsSelfReferral(userID, referrerID)
		if err != nil {
			log.Printf("校验推广关系失败，按不计提处理 [user=%d, referrer=%d]: %v", userID, referrerID, err)
			return "", 0, 0, false
		}
		if self {
			log.Printf("疑似自我推广，跳过提成计提 [user=%d, referrer=%d]", userID, referrerID)
			return "", 0, 0, false
		}
	}
	// 全局比例为 0 时视为该产品不提成
	rate := s.ProductRate(product)
	if rate <= 0 {
		return "", 0, 0, false
	}
	return referrerType, referrerID, rate, true
}

// writeCommission 写入一条提成流水：按推广身份类型分表（用户型推广 → user_commission，员工销售 → staff_commission）。
func (s *PromotionService) writeCommission(referrerType string, referrerID, userID int64, commission float64, bizType, refType string, refID int64, remark string) {
	if referrerID <= 0 || commission == 0 {
		return
	}
	table := model.SettlementTable(referrerType)
	tx, err := s.balance.db.Begin()
	if err != nil {
		log.Printf("推广提成记账失败（开启事务）[referrer=%s#%d, user=%d]: %v", referrerType, referrerID, userID, err)
		return
	}
	defer tx.Rollback()

	if err := s.promotionRepo.CreateSettlementTx(table, tx, &model.UserCommission{
		ReferrerType: referrerType,
		ReferrerID:   referrerID,
		UserID:       userID,
		Amount:       commission,
		BizType:      bizType,
		RefType:      refType,
		RefID:        refID,
		Remark:       remark,
	}); err != nil {
		log.Printf("推广提成记账失败 [referrer=%s#%d, user=%d, table=%s]: %v", referrerType, referrerID, userID, table, err)
		return
	}
	if err := tx.Commit(); err != nil {
		log.Printf("推广提成提交失败 [referrer=%s#%d, user=%d]: %v", referrerType, referrerID, userID, err)
	}
}

// chargedAmount 资源包计费实际金额为 0，余额计费为应付金额
func chargedAmount(payType int, amount float64) float64 {
	if payType == model.PayTypePack {
		return 0
	}
	return amount
}

// roundAmount 金额保留两位小数（避免浮点误差累积）
func roundAmount(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}
