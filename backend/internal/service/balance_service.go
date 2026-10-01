package service

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"sync"
	"time"

	"oemrpa/internal/audit"
	"oemrpa/internal/model"
	"oemrpa/internal/repository"
	"oemrpa/internal/upstream"
	"oemrpa/internal/utils"
)

var (
	ErrInsufficientBalance = errors.New("insufficient balance")
	// ErrDailyOnlineLimitExceeded 单用户单日在线支付金额超限（错误信息含限额与已用额度）
	ErrDailyOnlineLimitExceeded = errors.New("单日在线支付限额超出")
)

// generateBizNo 生成平台 20 位唯一业务号（账单专用）
func generateBizNo() string {
	return utils.GenerateRandomDigits(20)
}

// onlineChannelSupported 是否为平台支持的在线支付渠道
func onlineChannelSupported(channel string) bool {
	switch channel {
	case model.ChannelAlipay, model.ChannelWechat:
		return true
	}
	return false
}

// channelLabel 支付渠道中文名（用于账单备注）
func channelLabel(channel string) string {
	switch channel {
	case model.ChannelAlipay:
		return "支付宝"
	case model.ChannelWechat:
		return "微信支付"
	case model.ChannelManual:
		return "人工支付"
	}
	return channel
}

type BalanceService struct {
	userRepo             *repository.UserRepository
	billRepo             *repository.BillRepository
	paymentRepo          *repository.PaymentOrderRepository
	resourcePackRepo     *repository.ResourcePackRepository
	smsResourcePackRepo  *repository.SmsResourcePackRepository
	promotionRepo        *repository.PromotionRepository
	db                   *sql.DB
	paymentExpireMinutes int // 待支付订单过期分钟数（PAYMENT_EXPIRE_MINUTES，默认 30）

	reconcileMu     sync.Mutex          // 保护 lastReconcileAt
	lastReconcileAt map[int64]time.Time // 用户侧主动对账限流：orderID -> 最近一次主动对账时间

	// packPriceFn 资源包售价解析（推广/用户级覆盖优先），由 推广 服务在装配时注入；
	// 未注入时使用资源包自身价格。
	packPriceFn func(userID int64, product string, packID int64, def float64) float64
	// dailyOnlineLimitFn 单用户单日在线支付限额取数（后台系统设置可改，0 表示不限），装配时注入
	dailyOnlineLimitFn func() float64
	// commissionAccruer 资源包成交后的推广提成计提（由推广服务注入，避免 service 间直接依赖）：
	// service 为资源包所属服务标识（fv_auth / fv_self / sms），count 为包内次数/条数（用于按利润计提）
	commissionAccruer func(userID int64, service string, amount float64, count int64, refType string, refID int64, remark string)
}

// SetPackPriceResolver 注入资源包售价解析函数（在路由装配阶段调用）
func (s *BalanceService) SetPackPriceResolver(fn func(userID int64, product string, packID int64, def float64) float64) {
	s.packPriceFn = fn
}

// packPrice 返回用户适用的资源包售价（无解析器时用资源包原价）
func (s *BalanceService) packPrice(userID int64, product string, packID int64, def float64) float64 {
	if s.packPriceFn == nil {
		return def
	}
	return s.packPriceFn(userID, product, packID, def)
}

// SetCommissionAccruer 注入资源包成交后的推广提成计提函数（在路由装配阶段调用）
func (s *BalanceService) SetCommissionAccruer(fn func(userID int64, service string, amount float64, count int64, refType string, refID int64, remark string)) {
	s.commissionAccruer = fn
}

// accrueCommission 资源包成交后计提推广提成（未注入或无金额时跳过）
func (s *BalanceService) accrueCommission(userID int64, service string, amount float64, count int64, refType string, refID int64, remark string) {
	if s.commissionAccruer == nil || amount <= 0 {
		return
	}
	s.commissionAccruer(userID, service, amount, count, refType, refID, remark)
}

func NewBalanceService(
	userRepo *repository.UserRepository,
	billRepo *repository.BillRepository,
	paymentRepo *repository.PaymentOrderRepository,
	resourcePackRepo *repository.ResourcePackRepository,
	smsResourcePackRepo *repository.SmsResourcePackRepository,
	promotionRepo *repository.PromotionRepository,
	db *sql.DB,
	paymentExpireMinutes int,
) *BalanceService {
	if paymentExpireMinutes <= 0 {
		paymentExpireMinutes = 30
	}
	return &BalanceService{
		userRepo:             userRepo,
		billRepo:             billRepo,
		paymentRepo:          paymentRepo,
		resourcePackRepo:     resourcePackRepo,
		smsResourcePackRepo:  smsResourcePackRepo,
		promotionRepo:        promotionRepo,
		db:                   db,
		paymentExpireMinutes: paymentExpireMinutes,
		lastReconcileAt:      make(map[int64]time.Time),
	}
}

// writeBillTx 在事务中写入统一账单记录
func (s *BalanceService) writeBillTx(tx *sql.Tx, b *model.Bill) error {
	if b.BizNo == "" {
		b.BizNo = generateBizNo()
	}
	if err := s.billRepo.CreateTx(tx, b); err != nil {
		return err
	}
	auditBill(b)
	return nil
}

// writeBill 写入统一账单记录（非事务）
func (s *BalanceService) writeBill(b *model.Bill) error {
	if b.BizNo == "" {
		b.BizNo = generateBizNo()
	}
	if err := s.billRepo.Create(b); err != nil {
		return err
	}
	auditBill(b)
	return nil
}

// auditBill 资金变动审计（所有余额/充值/退款都会经过 writeBill，此处统一留痕，便于日后对账与回填）
func auditBill(b *model.Bill) {
	audit.Log("balance_change",
		audit.KV("bill_biz_no", b.BizNo),
		audit.KV("user_id", b.UserID),
		audit.KV("product", b.Product),
		audit.KV("service", b.Service),
		audit.KV("bill_type", b.BillType),
		audit.KV("spend_type", b.SpendType),
		audit.KV("pay_type", b.PayType),
		audit.KV("amount", fmt.Sprintf("%.4f", b.Amount)),
		audit.KV("balance_before", fmt.Sprintf("%.4f", b.BalanceBefore)),
		audit.KV("balance_after", fmt.Sprintf("%.4f", b.BalanceAfter)),
		audit.KV("ref_type", b.RefType),
		audit.KV("ref_id", b.RefID),
		audit.KV("ref_biz_no", b.RefBizNo),
		audit.KV("pay_order_id", b.PayOrderID),
		audit.KV("remark", b.Remark))
}

// productOf 服务标识映射到一级产品（fv/sms/kyc），用于账单 product 字段归类
func productOf(service string) string {
	switch service {
	case model.ServiceFVAuth, model.ServiceFVSelf:
		return "fv"
	case model.ServiceSMS:
		return "sms"
	case model.ServiceKYCPersonal, model.ServiceKYCEnterprise:
		return "kyc"
	}
	return service
}

// DeductBalance 扣除余额（预扣费），并写统一账单（spend_type=balance）
// product/service 标识所属产品与服务；refType 标识关联业务表类型（auth_record/sms_send_record 等）
func (s *BalanceService) DeductBalance(userID int64, amount float64, orderID int64, remark, product, service, refType string) error {
	// 开启事务
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 在事务中锁定并查询用户当前余额
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return err
	}

	// 检查余额是否充足
	if balance < amount {
		return ErrInsufficientBalance
	}

	// 扣除余额
	newBalance := balance - amount
	if err := s.userRepo.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}

	// 记录统一账单（余额直接扣费）
	bill := &model.Bill{
		UserID:        userID,
		Product:       product,
		Service:       service,
		BillType:      model.BillTypeConsume,
		SpendType:     model.SpendTypeBalance,
		PayType:       model.PayTypeBalance,
		Amount:        amount,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       refType,
		RefID:         orderID,
		Remark:        remark,
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return err
	}

	return tx.Commit()
}

// ---------- 两阶段下单：第一阶段「建未支付订单」 ----------

// PackOrderTarget 资源包下单目标。人脸核验包与短信包分属两库、主键会重复，故必须带产品标识区分。
type PackOrderTarget struct {
	Product    string // fv_auth / fv_self / sms / sms_marketing
	PackID     int64
	Name       string
	TotalCount int
	Price      float64
	IsSms      bool
}

// packBizNo 资源包订单的业务单号：`产品标识:资源包ID`（沿用 price_override.target 的既有约定）
func packBizNo(product string, packID int64) string {
	return fmt.Sprintf("%s:%d", product, packID)
}

// parsePackBizNo 解析资源包订单业务单号，返回产品标识与资源包 ID
func parsePackBizNo(bizNo string) (string, int64, error) {
	product, idStr, ok := strings.Cut(bizNo, ":")
	if !ok {
		return "", 0, fmt.Errorf("资源包订单业务单号非法: %s", bizNo)
	}
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil || id <= 0 {
		return "", 0, fmt.Errorf("资源包订单业务单号非法: %s", bizNo)
	}
	return product, id, nil
}

// resolvePackTarget 按产品定位资源包定义（人脸核验走 fv 库、短信走短信库），并解析用户适用的覆盖价
func (s *BalanceService) resolvePackTarget(userID int64, product string, packID int64) (*PackOrderTarget, error) {
	switch product {
	case model.ServiceFVAuth, model.ServiceFVSelf:
		pack, err := s.resourcePackRepo.GetPackByID(packID)
		if err != nil {
			return nil, err
		}
		if pack.Status != 1 {
			return nil, repository.ErrPackOffSale
		}
		return &PackOrderTarget{
			Product: pack.Product, PackID: pack.ID, Name: pack.Name, TotalCount: pack.TotalCount,
			Price: s.packPrice(userID, pack.Product, pack.ID, pack.Price),
		}, nil
	case model.ServiceSMS, model.ProductSMSMarketing:
		pack, err := s.smsResourcePackRepo.GetPackByID(packID)
		if err != nil {
			return nil, err
		}
		if pack.Status != 1 {
			return nil, repository.ErrPackOffSale
		}
		return &PackOrderTarget{
			Product: pack.Product, PackID: pack.ID, Name: pack.Name, TotalCount: pack.TotalCount,
			Price: s.packPrice(userID, pack.Product, pack.ID, pack.Price), IsSms: true,
		}, nil
	}
	return nil, fmt.Errorf("不支持的资源包类型: %s", product)
}

// packTargetForSettle 读取资源包定义用于订单落地：不校验上架状态、不解析覆盖价，
// 已支付订单须按购买时的包定义发放（包下架不影响已成交订单）。
func (s *BalanceService) packTargetForSettle(product string, packID int64) (*PackOrderTarget, error) {
	switch product {
	case model.ServiceFVAuth, model.ServiceFVSelf:
		pack, err := s.resourcePackRepo.GetPackByID(packID)
		if err != nil {
			return nil, err
		}
		return &PackOrderTarget{Product: pack.Product, PackID: pack.ID, Name: pack.Name, TotalCount: pack.TotalCount, Price: pack.Price}, nil
	case model.ServiceSMS, model.ProductSMSMarketing:
		pack, err := s.smsResourcePackRepo.GetPackByID(packID)
		if err != nil {
			return nil, err
		}
		return &PackOrderTarget{Product: pack.Product, PackID: pack.ID, Name: pack.Name, TotalCount: pack.TotalCount, Price: pack.Price, IsSms: true}, nil
	}
	return nil, fmt.Errorf("不支持的资源包类型: %s", product)
}

// CreateOrder 建未支付订单（第一阶段）：不扣余额、不选渠道、不调上游。
//   - intent=recharge：amount 为充值金额（须 > 0），biz_no 为空；
//   - intent=resource_pack：按 product + packID 定位资源包并解析用户适用覆盖价作为应付总额，
//     biz_no 为 `产品标识:资源包ID`。
//
// 同一用户同用途同业务单号同金额的未过期待支付单直接复用，避免重复建单。
func (s *BalanceService) CreateOrder(userID int64, intent, product string, packID int64, amount float64) (*model.PaymentOrder, error) {
	var bizNo string
	total := amount
	switch intent {
	case paymentIntentRecharge:
		if amount <= 0 {
			return nil, fmt.Errorf("充值金额须大于 0")
		}
	case paymentIntentResourcePack:
		target, err := s.resolvePackTarget(userID, product, packID)
		if err != nil {
			return nil, err
		}
		bizNo = packBizNo(target.Product, target.PackID)
		total = target.Price
	default:
		return nil, fmt.Errorf("不支持的订单用途: %s", intent)
	}

	if existing, err := s.paymentRepo.GetPendingOrder(userID, intent, bizNo, total); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}

	order := s.newPaymentOrder(userID, total, "", intent, bizNo, 0)
	if err := s.paymentRepo.CreateOrder(order); err != nil {
		return nil, err
	}
	return order, nil
}

// GetOrderForUser 查询属于指定用户的订单（支付页/轮询用），非本人订单一律视为不存在
func (s *BalanceService) GetOrderForUser(userID int64, payOrderNo string) (*model.PaymentOrder, error) {
	order, err := s.paymentRepo.GetOrderByPayOrderNo(payOrderNo)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, repository.ErrPaymentOrderNotFound
	}
	return order, nil
}

// ListUnpaidOrders 查询用户仍未过期的待支付订单（供「我的订单」页继续支付）
func (s *BalanceService) ListUnpaidOrders(userID int64) ([]*model.PaymentOrder, error) {
	return s.paymentRepo.ListUnpaidByUser(userID)
}

// refundBalanceTx 事务内退还余额并写退款账单（refType=payment_order）
func (s *BalanceService) refundBalanceTx(tx *sql.Tx, userID int64, amount float64, orderID int64, remark string) error {
	if amount <= 0 {
		return nil
	}
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return err
	}
	newBalance := balance + amount
	if err := s.userRepo.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}
	bill := &model.Bill{
		UserID:        userID,
		Service:       "refund",
		BillType:      model.BillTypeRefund,
		SpendType:     model.SpendTypeRefund,
		PayType:       model.PayTypeBalance,
		Amount:        amount,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       "payment_order",
		RefID:         orderID,
		PayOrderID:    orderID,
		Remark:        remark,
	}
	return s.writeBillTx(tx, bill)
}

// resetOrderBindingTx 事务内复位订单的渠道绑定与余额抵扣（重新选择支付方式/上游取链接失败时使用），
// 并退还上次已抵扣的余额。复位后订单回到「未选渠道的待支付单」形态。
func (s *BalanceService) resetOrderBindingTx(tx *sql.Tx, order *model.PaymentOrder, total float64) error {
	if order.BalanceAmount > 0 {
		if err := s.refundBalanceTx(tx, order.UserID, order.BalanceAmount, order.ID, "重新选择支付方式，退还上次抵扣余额"); err != nil {
			return err
		}
	}
	if err := s.paymentRepo.RevertOnlinePaymentTx(tx, order.ID, total); err != nil {
		return err
	}
	order.Amount, order.BalanceAmount, order.Channel, order.PayInfo = total, 0, "", ""
	return nil
}

// validatePayableOrder 校验订单可支付：属于本人、待支付、未过期
func validatePayableOrder(order *model.PaymentOrder, userID int64) error {
	if order.UserID != userID {
		return repository.ErrPaymentOrderNotFound
	}
	if order.Status != 0 {
		return fmt.Errorf("订单状态已变更，无法支付")
	}
	if order.ExpireTime != nil && order.ExpireTime.Before(time.Now()) {
		return fmt.Errorf("订单已过期，请重新下单")
	}
	return nil
}

// 支付用途：充值 / 购买资源包（对外可见，handler 与前端按此区分订单去向）
const (
	PaymentIntentRecharge     = "recharge"
	PaymentIntentResourcePack = "resource_pack"
)

const (
	paymentIntentRecharge     = PaymentIntentRecharge
	paymentIntentResourcePack = PaymentIntentResourcePack
)

// GetUserBalance 查询用户当前余额（订单页展示可用余额）
func (s *BalanceService) GetUserBalance(userID int64) (float64, error) {
	user, err := s.userRepo.GetUserByID(userID)
	if err != nil {
		return 0, err
	}
	return user.Balance, nil
}

// SetDailyOnlineLimitFn 注入单日在线支付限额取数函数（后台系统设置可改，0 表示不限）
func (s *BalanceService) SetDailyOnlineLimitFn(fn func() float64) {
	s.dailyOnlineLimitFn = fn
}

// dailyOnlineLimit 当前生效的单日在线支付限额
func (s *BalanceService) dailyOnlineLimit() float64 {
	if s.dailyOnlineLimitFn == nil {
		return 0
	}
	return s.dailyOnlineLimitFn()
}

// CheckDailyOnlineLimit 校验本次在线支付是否超出单用户单日限额（限额 <= 0 表示不限）。
// 统计口径为当日已支付与待支付的支付宝/微信订单金额之和。
func (s *BalanceService) CheckDailyOnlineLimit(userID int64, amount float64) error {
	limit := s.dailyOnlineLimit()
	if limit <= 0 || amount <= 0 {
		return nil
	}
	used, err := s.paymentRepo.SumDailyOnlineAmount(userID)
	if err != nil {
		return fmt.Errorf("统计当日在线支付金额失败: %w", err)
	}
	if used+amount > limit+1e-6 {
		return fmt.Errorf("%w：单用户单日在线支付限额 %.2f 元（今日已使用 %.2f 元），本次支付 %.2f 元将超出限额，请改用余额支付或明日再试",
			ErrDailyOnlineLimitExceeded, limit, used, amount)
	}
	return nil
}

// newPaymentOrder 构造待支付支付单（未落库）
func (s *BalanceService) newPaymentOrder(userID int64, amount float64, channel, intent, bizNo string, balanceAmount float64) *model.PaymentOrder {
	payOrderNo := fmt.Sprintf("R%s%06d", time.Now().Format("20060102150405"), time.Now().UnixNano()%1000000)
	expireTime := time.Now().Add(time.Duration(s.paymentExpireMinutes) * time.Minute)
	return &model.PaymentOrder{
		PayOrderNo:    payOrderNo,
		UserID:        userID,
		Amount:        amount,
		Channel:       channel,
		Status:        0, // 待支付
		RefundStatus:  0,
		ExpireTime:    &expireTime,
		Intent:        intent,
		BizNo:         bizNo,
		BalanceAmount: balanceAmount,
	}
}

// ---------- 两阶段下单：第二阶段「选定支付方式并支付」 ----------

// OnlinePaymentPrep 在线支付准备结果（余额抵扣已完成，等待调用方取上游支付链接）。
// ExternalPart == 0 表示余额已覆盖全款，调用方应改走 PayOrderWithBalance（余额支付）。
type OnlinePaymentPrep struct {
	Order         *model.PaymentOrder
	BalancePart   float64 // 本次余额抵扣部分
	ExternalPart  float64 // 需在线支付的差额
	BalanceBefore float64 // 扣减前的余额
}

// settleOrderByBalanceTx 事务内以余额全额结算资源包订单：扣余额 + 标记已支付 + 发放资源包 + 记账单。
// 提成与审计由调用方在事务提交后处理。
func (s *BalanceService) settleOrderByBalanceTx(
	tx *sql.Tx, order *model.PaymentOrder, target *PackOrderTarget, total float64,
) (refType string, refID int64, userPack *model.UserResourcePack, smsPack *model.SmsUserResourcePack, balanceBefore float64, err error) {
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, order.UserID)
	if err != nil {
		return "", 0, nil, nil, 0, err
	}
	if balance < total {
		return "", 0, nil, nil, 0, ErrInsufficientBalance
	}
	newBalance := balance - total
	if err := s.userRepo.UpdateUserBalanceTx(tx, order.UserID, newBalance); err != nil {
		return "", 0, nil, nil, 0, err
	}
	if _, err := s.paymentRepo.MarkOrderPaidWithChannelTx(tx, order.ID, model.ChannelBalance, 0, total); err != nil {
		return "", 0, nil, nil, 0, err
	}

	if target.IsSms {
		smsPack = &model.SmsUserResourcePack{
			UserID:         order.UserID,
			TotalCount:     target.TotalCount,
			RemainingCount: target.TotalCount,
			Product:        target.Product, // 类型快照：消费端按此与模板类型匹配
			Price:          total,          // 价格快照：实付金额
			Status:         1,
		}
		if err := s.smsResourcePackRepo.CreateUserPackTx(tx, smsPack); err != nil {
			return "", 0, nil, nil, 0, err
		}
		refType, refID = "sms_user_resource_pack", smsPack.ID
	} else {
		userPack = &model.UserResourcePack{
			UserID:         order.UserID,
			TotalCount:     target.TotalCount,
			RemainingCount: target.TotalCount,
			Product:        target.Product, // 产品快照：消费端按此匹配资源包，缺省会导致已购包无法抵扣
			Price:          total,          // 价格快照：实付金额
			Status:         1,
		}
		if err := s.resourcePackRepo.CreateUserPackTx(tx, userPack); err != nil {
			return "", 0, nil, nil, 0, err
		}
		refType, refID = "user_resource_pack", userPack.ID
	}

	bill := &model.Bill{
		UserID:        order.UserID,
		Product:       target.Product,
		Service:       target.Product,
		BillType:      model.BillTypeConsume,
		SpendType:     model.SpendTypePackPurchase,
		PayType:       model.PayTypeBalance,
		Amount:        total,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       refType,
		RefID:         refID,
		PayOrderID:    order.ID,
		Remark:        fmt.Sprintf("购买资源包：%s（余额支付）", target.Name),
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return "", 0, nil, nil, 0, err
	}
	return refType, refID, userPack, smsPack, balance, nil
}

// PayOrderWithBalance 以余额全额支付资源包订单：原子完成「扣余额 + 标记已支付 + 发放资源包 + 记账单」。
// 充值订单不支持余额支付；订单此前若已绑定在线渠道，先退还已抵扣余额再按全额扣减。
func (s *BalanceService) PayOrderWithBalance(userID int64, payOrderNo string) (*model.UserResourcePack, *model.SmsUserResourcePack, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback()

	order, err := s.paymentRepo.GetOrderByPayOrderNoForUpdateTx(tx, payOrderNo)
	if err != nil {
		return nil, nil, err
	}
	if err := validatePayableOrder(order, userID); err != nil {
		return nil, nil, err
	}
	if order.Intent != paymentIntentResourcePack {
		return nil, nil, fmt.Errorf("充值订单不支持余额支付")
	}
	product, packID, err := parsePackBizNo(order.BizNo)
	if err != nil {
		return nil, nil, err
	}
	target, err := s.resolvePackTarget(userID, product, packID)
	if err != nil {
		return nil, nil, err
	}

	total := order.Amount + order.BalanceAmount
	if order.Channel != "" || order.BalanceAmount > 0 {
		// 重新选择支付方式：先退还上次抵扣的余额并复位订单
		if err := s.resetOrderBindingTx(tx, order, total); err != nil {
			return nil, nil, err
		}
	}

	refType, refID, userPack, smsPack, _, err := s.settleOrderByBalanceTx(tx, order, target, total)
	if err != nil {
		return nil, nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, nil, err
	}

	s.accrueCommission(userID, target.Product, total, int64(target.TotalCount), refType, refID, fmt.Sprintf("购买资源包：%s", target.Name))
	audit.Log("pack_issue",
		audit.KV("user_pack_id", refID),
		audit.KV("user_id", userID),
		audit.KV("pack_id", target.PackID),
		audit.KV("product", target.Product),
		audit.KV("total_count", target.TotalCount),
		audit.KV("pay_order_no", order.PayOrderNo),
		audit.KV("channel", model.ChannelBalance))
	return userPack, smsPack, nil
}

// PrepareOrderOnlinePayment 在线支付准备（第二阶段）：事务内完成余额抵扣与渠道绑定，返回在线支付差额。
// useBalance 为 true 时自动优先抵扣余额（抵扣额与差额随订单一并落库）。
// ExternalPart == 0 表示余额已覆盖全款，调用方应改走 PayOrderWithBalance。
// 调用方取得上游支付链接后调 BindOrderPayInfo 回填；上游失败调 RevertOrderOnlinePayment 回滚。
func (s *BalanceService) PrepareOrderOnlinePayment(userID int64, payOrderNo, channel string, useBalance bool) (*OnlinePaymentPrep, error) {
	if !onlineChannelSupported(channel) {
		return nil, fmt.Errorf("不支持的支付渠道: %s", channel)
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	order, err := s.paymentRepo.GetOrderByPayOrderNoForUpdateTx(tx, payOrderNo)
	if err != nil {
		return nil, err
	}
	if err := validatePayableOrder(order, userID); err != nil {
		return nil, err
	}

	total := order.Amount + order.BalanceAmount
	if order.Channel != "" || order.BalanceAmount > 0 {
		// 重新选择支付方式：先退还上次抵扣的余额并复位订单
		if err := s.resetOrderBindingTx(tx, order, total); err != nil {
			return nil, err
		}
	}

	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return nil, err
	}
	// 充值订单不允许用余额抵扣（用余额给自己充值无意义），仅购买资源包可抵扣
	balancePart := 0.0
	if useBalance && order.Intent == paymentIntentResourcePack && balance > 0 {
		balancePart = math.Min(balance, total)
	}
	externalPart := total - balancePart

	// 余额已覆盖全款：不改动任何状态，交由调用方改走余额支付
	if externalPart <= 0 {
		return &OnlinePaymentPrep{Order: order, BalancePart: total, ExternalPart: 0, BalanceBefore: balance}, nil
	}

	// 单日在线支付限额：按本次实际在线支付金额校验
	if err := s.CheckDailyOnlineLimit(userID, externalPart); err != nil {
		return nil, err
	}

	if balancePart > 0 {
		if err := s.userRepo.UpdateUserBalanceTx(tx, userID, balance-balancePart); err != nil {
			return nil, err
		}
	}
	changed, err := s.paymentRepo.BindOnlinePaymentTx(tx, order.ID, channel, "", externalPart, balancePart)
	if err != nil {
		return nil, err
	}
	if !changed {
		return nil, fmt.Errorf("订单状态已变更，无法支付")
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	order.Amount, order.BalanceAmount, order.Channel = externalPart, balancePart, channel
	return &OnlinePaymentPrep{Order: order, BalancePart: balancePart, ExternalPart: externalPart, BalanceBefore: balance}, nil
}

// BindOrderPayInfo 回填订单的渠道支付信息（上游取支付链接成功后）
func (s *BalanceService) BindOrderPayInfo(orderID int64, payInfo string) error {
	return s.paymentRepo.UpdatePayInfo(orderID, payInfo)
}

// RevertOrderOnlinePayment 上游取支付链接失败时回滚：退还本次抵扣余额并复位订单为未选渠道的待支付单
func (s *BalanceService) RevertOrderOnlinePayment(userID int64, payOrderNo string, total float64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	order, err := s.paymentRepo.GetOrderByPayOrderNoForUpdateTx(tx, payOrderNo)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return repository.ErrPaymentOrderNotFound
	}
	if order.Status != 0 {
		return nil
	}
	if err := s.resetOrderBindingTx(tx, order, total); err != nil {
		return err
	}
	return tx.Commit()
}

// CancelOrder 取消待支付订单（0→3），并退还已抵扣的余额
func (s *BalanceService) CancelOrder(userID int64, payOrderNo string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	order, err := s.paymentRepo.GetOrderByPayOrderNoForUpdateTx(tx, payOrderNo)
	if err != nil {
		return err
	}
	if order.UserID != userID {
		return repository.ErrPaymentOrderNotFound
	}
	if order.Status != 0 {
		return fmt.Errorf("订单状态已变更，无法取消")
	}
	changed, err := s.paymentRepo.CloseOrderIfPendingTx(tx, order.ID)
	if err != nil {
		return err
	}
	if !changed {
		return tx.Commit()
	}
	if order.BalanceAmount > 0 {
		if err := s.refundBalanceTx(tx, order.UserID, order.BalanceAmount, order.ID, "取消订单，退还抵扣余额"); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// SettleResourcePackPaid 支付成功落地资源包订单：原子完成「标记已支付 + 发放资源包」，幂等（同时刻仅一次成功）
// 若订单已被关闭（超时/对账）但渠道确认已支付，则转为充值入账（金额冲入余额，用途改写为 recharge）。
func (s *BalanceService) SettleResourcePackPaid(orderID int64, channelTradeNo string) error {
	order, err := s.paymentRepo.GetOrderByID(orderID)
	if err != nil {
		return err
	}
	if order.Status == 3 {
		// 订单已关闭但用户仍完成支付：金额冲入余额，支付记录用途由购买产品修改为充值余额
		if _, err := s.SettleLatePaidClosedOrder(order, channelTradeNo); err != nil {
			return fmt.Errorf("已关闭资源包订单转充值入账失败: %w", err)
		}
		return nil
	}
	product, packID, err := parsePackBizNo(order.BizNo)
	if err != nil {
		return err
	}
	target, err := s.packTargetForSettle(product, packID)
	if err != nil {
		return err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	changed, err := s.paymentRepo.MarkOrderPaidIfPendingTx(tx, orderID, channelTradeNo)
	if err != nil {
		return err
	}
	if !changed {
		// 已处理过（幂等），直接成功
		return tx.Commit()
	}

	total := order.Amount + order.BalanceAmount // 价格快照：第三方支付部分 + 余额支付部分
	refType := "user_resource_pack"
	var refID int64
	if target.IsSms {
		up := &model.SmsUserResourcePack{
			UserID:         order.UserID,
			TotalCount:     target.TotalCount,
			RemainingCount: target.TotalCount,
			Product:        target.Product, // 类型快照：消费端按此与模板类型匹配
			Price:          total,
			Status:         1,
		}
		if err := s.smsResourcePackRepo.CreateUserPackTx(tx, up); err != nil {
			return err
		}
		refType, refID = "sms_user_resource_pack", up.ID
	} else {
		up := &model.UserResourcePack{
			UserID:         order.UserID,
			TotalCount:     target.TotalCount,
			RemainingCount: target.TotalCount,
			Product:        target.Product, // 产品快照：消费端按此匹配资源包，缺省会导致已购包无法抵扣
			Price:          total,
			Status:         1,
		}
		if err := s.resourcePackRepo.CreateUserPackTx(tx, up); err != nil {
			return err
		}
		refType, refID = "user_resource_pack", up.ID
	}
	// 记录统一账单（组合支付：第三方支付部分落地补账）
	bill := &model.Bill{
		UserID:        order.UserID,
		Product:       target.Product,
		Service:       target.Product,
		BillType:      model.BillTypeConsume,
		SpendType:     model.SpendTypePackPurchase,
		PayType:       model.PayTypeOfChannel(order.Channel),
		Amount:        order.Amount,
		BalanceBefore: order.BalanceAmount,
		BalanceAfter:  order.BalanceAmount,
		RefType:       refType,
		RefID:         refID,
		PayOrderID:    order.ID,
		Remark:        fmt.Sprintf("购买资源包：%s（%s）", target.Name, channelLabel(order.Channel)),
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}

	// 推广提成：按本次支付的全部金额（余额部分 + 第三方支付部分，扣除成本后）计提给用户归属推广商
	s.accrueCommission(order.UserID, target.Product, total, int64(target.TotalCount), refType, refID, fmt.Sprintf("购买资源包：%s", target.Name))
	audit.Log("pack_issue",
		audit.KV("user_pack_id", refID),
		audit.KV("user_id", order.UserID),
		audit.KV("pack_id", target.PackID),
		audit.KV("product", target.Product),
		audit.KV("total_count", target.TotalCount),
		audit.KV("pay_order_no", order.PayOrderNo),
		audit.KV("channel", order.Channel))
	return nil
}

// SettleLatePaidClosedOrder 处理「订单已关闭但用户仍完成支付」的场景（幂等）：
// 将已关闭订单标记为已支付并改写用途为 recharge，再把支付金额冲入用户余额。
// 状态改写与余额入账在同一事务内完成：任一步失败整体回滚，渠道重试时仍可正确入账，避免资金丢失。
// 返回是否发生了入账（已处理过的订单返回 false）。
func (s *BalanceService) SettleLatePaidClosedOrder(order *model.PaymentOrder, channelTradeNo string) (bool, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback()

	// 锁定订单行，防止并发重复入账
	locked, err := s.paymentRepo.GetOrderByIDForUpdateTx(tx, order.ID)
	if err != nil {
		return false, err
	}
	if locked.Status != 3 {
		// 已被并发处理为已支付（幂等），无需重复入账
		return false, tx.Commit()
	}

	changed, err := s.paymentRepo.SettleClosedOrderAsRechargeTx(tx, order.ID, channelTradeNo)
	if err != nil {
		return false, err
	}
	if !changed {
		return false, tx.Commit()
	}

	// 同事务内增加余额并记账
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, order.UserID)
	if err != nil {
		return false, err
	}
	newBalance := balance + order.Amount
	if err := s.userRepo.UpdateUserBalanceTx(tx, order.UserID, newBalance); err != nil {
		return false, err
	}
	bill := &model.Bill{
		UserID:        order.UserID,
		Product:       "",
		Service:       "recharge",
		BillType:      model.BillTypeRecharge,
		SpendType:     model.SpendTypeRecharge,
		PayType:       model.PayTypeOfChannel(order.Channel),
		Amount:        order.Amount,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       "payment_order",
		RefID:         order.ID,
		PayOrderID:    order.ID,
		Remark:        "已关闭订单完成支付，金额冲入余额",
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// QueryPaymentResult 管理后台手动查询单笔待支付单的渠道真实状态并落地（幂等）：
// 复用对账单笔逻辑（渠道已支付→补账/发放；渠道已关闭→关闭本地单），返回查询后的订单状态说明。
func (s *BalanceService) QueryPaymentResult(orderID int64, alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient) (string, error) {
	o, err := s.paymentRepo.GetOrderByID(orderID)
	if err != nil {
		return "", err
	}
	if o.Status != 0 {
		return "", fmt.Errorf("订单已处理（status=%d），无需查询", o.Status)
	}
	s.reconcileOne(o, alipay, wechatPay, true)

	after, err := s.paymentRepo.GetOrderByID(orderID)
	if err != nil {
		return "", err
	}
	switch after.Status {
	case 1:
		return "渠道已支付，已补账/发放", nil
	case 3:
		return "渠道未支付或已关闭，本地订单已关闭", nil
	default:
		return "渠道查询结果：待支付", nil
	}
}

// ReleaseExpiredPaymentOrders 超时撤回支付（定时任务每分钟调用）：
// 扫描所有已过期仍待支付的订单，先向渠道关单撤回（支付宝/微信，尽力而为），再关闭本地订单；
// 资源包订单额外退还余额支付部分（幂等）。
func (s *BalanceService) ReleaseExpiredPaymentOrders(alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient) error {
	orders, err := s.paymentRepo.GetExpiredPendingOrders(time.Now())
	if err != nil {
		return err
	}
	for _, o := range orders {
		// 渠道侧撤回（关单），失败不影响本地关闭
		switch o.Channel {
		case model.ChannelAlipay:
			if alipay != nil {
				if cerr := alipay.CloseOrder(o.PayOrderNo); cerr != nil {
					log.Printf("支付宝撤回支付失败 [pay_order_no=%s]: %v", o.PayOrderNo, cerr)
				}
			}
		case model.ChannelWechat:
			if wechatPay != nil {
				if cerr := wechatPay.CloseOrder(o.PayOrderNo); cerr != nil {
					log.Printf("微信撤回支付失败 [pay_order_no=%s]: %v", o.PayOrderNo, cerr)
				}
			}
		}
		s.closeExpiredOrder(o)
	}
	return nil
}

// closeExpiredOrder 关闭单笔超时待支付订单（资源包复用关闭逻辑：关闭+退还余额部分；充值仅关闭）
func (s *BalanceService) closeExpiredOrder(o *model.PaymentOrder) {
	if o.Intent == paymentIntentResourcePack {
		if err := s.closeResourcePackOrder(o); err != nil {
			log.Printf("关闭超时资源包订单失败 [order_id=%d]: %v", o.ID, err)
		}
		return
	}
	if changed, err := s.paymentRepo.CloseOrderIfPending(o.ID); err == nil && changed {
		log.Printf("关闭超时充值订单: pay_order_no=%s", o.PayOrderNo)
	}
}

// closeResourcePackOrder 关闭单个资源包待支付订单，事务内原子关闭（幂等），退还余额支付部分
// 返回错误时未完成关闭（或未退还余额），调用方应中止后续依赖该结果的操作
func (s *BalanceService) closeResourcePackOrder(order *model.PaymentOrder) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("开启事务失败: %w", err)
	}
	defer tx.Rollback()

	// 仅关闭仍为待支付的订单，避免与支付回调竞争导致重复退款
	changed, err := s.paymentRepo.CloseOrderIfPendingTx(tx, order.ID)
	if err != nil {
		return err
	}
	if !changed {
		// 已被并发关闭/支付，无需处理
		return tx.Commit()
	}

	// 退还余额支付部分
	if order.BalanceAmount > 0 {
		balance, err := s.userRepo.GetBalanceForUpdateTx(tx, order.UserID)
		if err != nil {
			return fmt.Errorf("查询余额失败: %w", err)
		}
		newBalance := balance + order.BalanceAmount
		if err := s.userRepo.UpdateUserBalanceTx(tx, order.UserID, newBalance); err != nil {
			return fmt.Errorf("退还余额失败: %w", err)
		}
		// 记录统一账单（资源包订单超时退款）
		refundBill := &model.Bill{
			UserID:        order.UserID,
			Product:       "",
			Service:       "refund",
			BillType:      model.BillTypeRefund,
			SpendType:     model.SpendTypeRefund,
			PayType:       model.PayTypeBalance,
			Amount:        order.BalanceAmount,
			BalanceBefore: balance,
			BalanceAfter:  newBalance,
			RefType:       "payment_order",
			RefID:         order.ID,
			PayOrderID:    order.ID,
			Remark:        fmt.Sprintf("购买资源包订单超时退款：%s", order.BizNo),
		}
		if err := s.writeBillTx(tx, refundBill); err != nil {
			return fmt.Errorf("写退款账单失败: %w", err)
		}
	}

	return tx.Commit()
}

// ReconcilePaymentOrders 支付对账（每日定时任务调用）：主动向支付渠道查询仍待支付订单的真实状态并落地
// 仅处理创建超 5 分钟的待支付单，避免干扰正在正常支付流程中的订单
func (s *BalanceService) ReconcilePaymentOrders(alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient) {
	orders, err := s.paymentRepo.GetPendingOrdersForReconcile(time.Now().Add(-5 * time.Minute))
	if err != nil {
		log.Printf("支付对账查询待支付订单失败: %v", err)
		return
	}
	for _, o := range orders {
		s.reconcileOne(o, alipay, wechatPay, true)
	}
}

// reconcileOne 对单笔待支付订单向对应渠道查询真实状态并补账/关闭（幂等）。
// closeWhenUnpaid 为 true 时（定时对账/管理员查询）渠道未支付或已关闭则关闭本地单；
// 为 false 时（用户侧查询支付结果）仅在渠道确认已支付时补账/发放，未支付保持待支付，避免打断正在进行的支付。
func (s *BalanceService) reconcileOne(o *model.PaymentOrder, alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient, closeWhenUnpaid bool) {
	switch o.Channel {
	case model.ChannelAlipay:
		if alipay == nil {
			return
		}
		res, err := alipay.QueryOrder(o.PayOrderNo)
		if err != nil {
			log.Printf("支付宝对账查询失败 [pay_order_no=%s]: %v", o.PayOrderNo, err)
			return
		}
		tr := res.AlipayTradeQueryResponse
		switch {
		case tr.Code == "10000" && (tr.TradeStatus == "TRADE_SUCCESS" || tr.TradeStatus == "TRADE_FINISHED"):
			s.settleReconciledPaid(o, tr.TradeNo)
		case tr.Code == "10000" && tr.TradeStatus == "TRADE_CLOSED":
			if closeWhenUnpaid {
				s.closeReconciledOrder(o)
			}
		case tr.SubCode == "ACQ.TRADE_NOT_EXIST":
			// 支付宝侧从未生成交易（用户未扫码）：订单未过期保持待支付，已过期由超时撤回任务关闭
		default:
			log.Printf("支付宝对账业务异常 [pay_order_no=%s]: code=%s sub_code=%s msg=%s", o.PayOrderNo, tr.Code, tr.SubCode, tr.Msg)
		}
	case model.ChannelWechat:
		if wechatPay == nil {
			return
		}
		tradeState, transactionID, err := wechatPay.QueryOrder(o.PayOrderNo)
		if err != nil {
			log.Printf("微信对账查询失败 [pay_order_no=%s]: %v", o.PayOrderNo, err)
			return
		}
		switch tradeState {
		case "SUCCESS":
			s.settleReconciledPaid(o, transactionID)
		case "CLOSED", "NOTPAY", "PAYERROR":
			if closeWhenUnpaid {
				s.closeReconciledOrder(o)
			}
		}
	}
}

// ReconcilePendingIfStale 用户侧查询支付结果：对待支付单主动向渠道确认真实状态并落地（幂等）。
// 仅在渠道确认已支付时补账/发放，未支付保持待支付（不关闭本地单）；同一订单 30 秒内最多主动对账一次，
// 避免前端轮询高频调用渠道接口。返回查询后的最新订单。
func (s *BalanceService) ReconcilePendingIfStale(order *model.PaymentOrder, alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient) (*model.PaymentOrder, error) {
	if order.Status != 0 {
		return order, nil
	}
	s.reconcileMu.Lock()
	last, ok := s.lastReconcileAt[order.ID]
	if ok && time.Since(last) < 30*time.Second {
		s.reconcileMu.Unlock()
		return order, nil
	}
	s.lastReconcileAt[order.ID] = time.Now()
	s.reconcileMu.Unlock()

	s.reconcileOne(order, alipay, wechatPay, false)
	fresh, err := s.paymentRepo.GetOrderByID(order.ID)
	if err != nil {
		return nil, err
	}
	return fresh, nil
}

// settleReconciledPaid 对账确认已支付时补账（幂等：与支付回调共用同一套状态条件更新）
func (s *BalanceService) settleReconciledPaid(o *model.PaymentOrder, channelTradeNo string) {
	if o.Intent == paymentIntentResourcePack {
		if err := s.SettleResourcePackPaid(o.ID, channelTradeNo); err != nil {
			log.Printf("对账补单（资源包）失败 [pay_order_no=%s]: %v", o.PayOrderNo, err)
		}
		return
	}
	changed, err := s.paymentRepo.MarkOrderPaidIfPending(o.ID, channelTradeNo)
	if err != nil {
		log.Printf("对账补单标记失败 [pay_order_no=%s]: %v", o.PayOrderNo, err)
		return
	}
	if !changed {
		// 状态未被更新：可能已被处理为已支付，也可能已被超时/对账关闭（此时渠道确认已支付则转充值入账）
		fresh, ferr := s.paymentRepo.GetOrderByID(o.ID)
		if ferr != nil {
			log.Printf("对账补单查询订单状态失败 [pay_order_no=%s]: %v", o.PayOrderNo, ferr)
			return
		}
		if fresh.Status == 3 {
			if credited, cerr := s.SettleLatePaidClosedOrder(fresh, channelTradeNo); cerr != nil {
				log.Printf("对账补单（已关闭订单转充值）失败 [pay_order_no=%s]: %v", o.PayOrderNo, cerr)
			} else if credited {
				log.Printf("对账补单（已关闭订单转充值入账）: pay_order_no=%s, amount=%.2f", o.PayOrderNo, o.Amount)
			}
		}
		return
	}
	if err := s.RechargeBalance(o.UserID, o.Amount, o.ID, o.Channel); err != nil {
		log.Printf("对账补单入账失败 [pay_order_no=%s]: %v", o.PayOrderNo, err)
		return
	}
	log.Printf("对账补单成功: pay_order_no=%s, amount=%.2f", o.PayOrderNo, o.Amount)
}

// closeReconciledOrder 对账确认未支付（渠道侧已关闭）时关闭本地订单
// 资源包订单复用超时关闭逻辑（关闭 + 退还余额部分），普通充值单仅关闭
func (s *BalanceService) closeReconciledOrder(o *model.PaymentOrder) {
	if o.Intent == paymentIntentResourcePack {
		if err := s.closeResourcePackOrder(o); err != nil {
			log.Printf("对账关闭资源包订单失败 [pay_order_no=%s]: %v", o.PayOrderNo, err)
		}
		return
	}
	changed, err := s.paymentRepo.CloseOrderIfPending(o.ID)
	if err != nil {
		log.Printf("对账关闭订单失败 [pay_order_no=%s]: %v", o.PayOrderNo, err)
		return
	}
	if changed {
		log.Printf("对账关闭订单: pay_order_no=%s", o.PayOrderNo)
	}
}

// GetPaymentOrder 根据支付订单号查询订单
func (s *BalanceService) GetPaymentOrder(payOrderNo string) (*model.PaymentOrder, error) {
	return s.paymentRepo.GetOrderByPayOrderNo(payOrderNo)
}

// WithdrawResult 单笔支付订单的提现（退款）结果
type WithdrawResult struct {
	OrderID      int64   `json:"order_id"`
	PayOrderNo   string  `json:"pay_order_no"`
	RefundAmount float64 `json:"refund_amount"`
}

// GetRefundableOrders 查询用户可提现（可退款）的已支付充值订单（按创建时间升序）
func (s *BalanceService) GetRefundableOrders(userID int64) ([]*model.PaymentOrder, error) {
	return s.paymentRepo.GetRefundableOrders(userID)
}

// Withdraw 提现：将用户余额按充值支付订单原路退回（支付宝/微信退款），余额同步扣减。
// 提现可部分退款单个订单，也可自动拆分多个订单覆盖全额（如提现 200 元，单笔订单不足时分多笔退款）；
// 每笔退款累计不超过对应订单剩余可退款额（订单金额 - 已退款额）。
// 返回已完成的各订单退款明细；存在不足或退款失败时返回错误（已成功的部分退款不自动回滚）。
func (s *BalanceService) Withdraw(userID int64, amount float64, alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient) ([]WithdrawResult, error) {
	if amount <= 0 {
		return nil, fmt.Errorf("提现金额必须大于 0")
	}
	// 余额必须足以覆盖提现金额（提现即时从余额扣减）
	balance, err := s.userRepo.GetBalance(userID)
	if err != nil {
		return nil, err
	}
	if balance+0.005 < amount {
		return nil, ErrInsufficientBalance
	}

	orders, err := s.paymentRepo.GetRefundableOrders(userID)
	if err != nil {
		return nil, err
	}

	remaining := amount
	results := make([]WithdrawResult, 0, 1)
	for _, o := range orders {
		if remaining <= 0 {
			break
		}
		refundable := o.Amount - o.RefundAmount
		if refundable <= 0 {
			continue
		}
		refund := refundable
		if refund > remaining {
			refund = remaining
		}
		if err := s.refundOrder(userID, o.ID, refund, alipay, wechatPay); err != nil {
			return results, fmt.Errorf("支付单 %s 退款失败: %w", o.PayOrderNo, err)
		}
		remaining -= refund
		results = append(results, WithdrawResult{OrderID: o.ID, PayOrderNo: o.PayOrderNo, RefundAmount: refund})
	}
	if remaining > 0 {
		return results, fmt.Errorf("可退款支付订单不足以覆盖提现金额，本次已退款 %.2f 元", amount-remaining)
	}
	return results, nil
}

// refundOrder 单笔支付订单提现退款：事务内锁定订单行与余额 → 调上游退款 → 更新订单退款记录并扣减余额（原子）。
// 事务持有行锁期间调用上游退款接口（同一订单的并发退款会被串行化，避免重复退款超限）。
func (s *BalanceService) refundOrder(userID, orderID int64, refund float64, alipay *upstream.AlipayClient, wechatPay *upstream.WechatPayClient) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 锁定支付订单行，重新校验剩余可退款与归属（并发安全）
	cur, err := s.paymentRepo.GetOrderByIDForUpdateTx(tx, orderID)
	if err != nil {
		return err
	}
	if cur.UserID != userID || cur.Status != 1 || cur.Intent != paymentIntentRecharge {
		return fmt.Errorf("订单不可退款（状态/归属/用途不符）")
	}
	if cur.Amount-cur.RefundAmount+0.005 < refund {
		return fmt.Errorf("订单剩余可退款 %.2f 元，不足本次提现 %.2f 元", cur.Amount-cur.RefundAmount, refund)
	}

	// 锁定余额并校验
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return err
	}
	if balance+0.005 < refund {
		return ErrInsufficientBalance
	}
	newBalance := balance - refund
	if err := s.userRepo.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}

	// 调上游退款（原路退回）；失败则整体回滚，不影响下次提现
	refundNo := generateRefundNo()
	switch cur.Channel {
	case model.ChannelAlipay:
		if alipay == nil {
			return fmt.Errorf("支付宝支付未配置，无法退款")
		}
		if err := alipay.RefundOrder(cur.PayOrderNo, refund, refundNo); err != nil {
			return err
		}
	case model.ChannelWechat:
		if wechatPay == nil {
			return fmt.Errorf("微信支付未配置，无法退款")
		}
		if err := wechatPay.Refund(cur.PayOrderNo, refundNo, refund, cur.Amount); err != nil {
			return err
		}
	default:
		return fmt.Errorf("不支持的支付渠道: %s", cur.Channel)
	}

	// 更新支付订单退款累计与状态
	newRefundAmount := cur.RefundAmount + refund
	refundStatus := 1 // 部分退款
	if newRefundAmount+0.005 >= cur.Amount {
		refundStatus = 2 // 全额退款
	}
	if err := s.paymentRepo.UpdateRefundTx(tx, cur.ID, newRefundAmount, refundStatus); err != nil {
		return err
	}

	// 统一账单（退款）
	bill := &model.Bill{
		UserID:        userID,
		Product:       "",
		Service:       "withdraw",
		BillType:      model.BillTypeRefund,
		SpendType:     model.SpendTypeRefund,
		PayType:       model.PayTypeOfChannel(cur.Channel),
		Amount:        refund,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       "payment_order",
		RefID:         cur.ID,
		PayOrderID:    cur.ID,
		Remark:        fmt.Sprintf("提现退款：%s", cur.PayOrderNo),
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return err
	}

	return tx.Commit()
}

// generateRefundNo 生成商户退款请求号（支付宝 out_request_no / 微信 out_refund_no，需唯一）
func generateRefundNo() string {
	return "RF" + utils.GenerateRandomDigits(18)
}

// ManualRechargeBalance 后台人工充值（人工支付）：同一事务内生成人工支付记录（渠道 manual、已支付状态）
// → 增加用户余额 → 记录统一账单（充值入账，绑定该支付单）。
// bankSerialNo 为银行流水单号（必填，全局唯一），用于幂等校验与对账。
func (s *BalanceService) ManualRechargeBalance(userID int64, amount float64, remark, bankSerialNo string) error {
	// 银行流水单号幂等：同一流水号只允许入账一次，防止重复提交重复加余额
	if bankSerialNo != "" {
		exists, err := s.paymentRepo.BankSerialExists(bankSerialNo)
		if err != nil {
			return fmt.Errorf("校验银行流水单号失败: %w", err)
		}
		if exists {
			return fmt.Errorf("银行流水单号 %s 已使用，请勿重复提交", bankSerialNo)
		}
	}

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 人工支付记录：无渠道流水号，直接置为已支付并记录支付时间
	now := time.Now()
	order := &model.PaymentOrder{
		PayOrderNo:   fmt.Sprintf("M%s%06d", now.Format("20060102150405"), now.UnixNano()%1000000),
		UserID:       userID,
		Amount:       amount,
		Channel:      "manual",
		BankSerialNo: bankSerialNo,
		Status:       1,
		PaidAt:       &now,
		Intent:       paymentIntentRecharge,
	}
	if err := s.paymentRepo.CreateManualPaymentTx(tx, order); err != nil {
		return fmt.Errorf("创建人工支付记录失败: %w", err)
	}

	// 锁定余额并增加
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return err
	}
	newBalance := balance + amount
	if err := s.userRepo.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}

	// 记录统一账单（充值入账，绑定人工支付单）
	bill := &model.Bill{
		UserID:        userID,
		Product:       "",
		Service:       "recharge",
		BillType:      model.BillTypeRecharge,
		SpendType:     model.SpendTypeRecharge,
		PayType:       model.PayTypeBalance,
		Amount:        amount,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       "payment_order",
		RefID:         order.ID,
		PayOrderID:    order.ID,
		Remark:        remark,
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return err
	}

	return tx.Commit()
}

// 测试资源包默认条数与单次发放上限（管理员未填写条数时取默认值）
const (
	testPackDefaultFv  = 5
	testPackDefaultSms = 20
	testPackMaxCount   = 1000
)

// TestPackGrant 测试资源包发放结果
type TestPackGrant struct {
	UserPackID int64
	Product    string
	Count      int
}

// GrantTestPack 管理员向指定用户发放测试资源包（免费发放，不产生资金账单，故不写 bill）。
// product 取 fv_auth（有源人脸核验）/ fv_self（无源人脸核验）/ sms（验证码/通知短信）/
// sms_marketing（营销短信），资源包按该类型精确匹配消费端，各类型互不通用；
// count <= 0 时取默认值，超过上限按上限截断。
func (s *BalanceService) GrantTestPack(adminID, userID int64, product string, count int) (*TestPackGrant, error) {
	switch product {
	case model.ServiceFVAuth, model.ServiceFVSelf, model.ServiceSMS, model.ProductSMSMarketing:
	default:
		return nil, fmt.Errorf("不支持的测试资源包类型: %s", product)
	}
	if count <= 0 {
		if product == model.ServiceSMS || product == model.ProductSMSMarketing {
			count = testPackDefaultSms
		} else {
			count = testPackDefaultFv
		}
	}
	if count > testPackMaxCount {
		count = testPackMaxCount
	}
	if _, err := s.userRepo.GetUserByID(userID); err != nil {
		if errors.Is(err, repository.ErrUserNotFound) {
			return nil, fmt.Errorf("用户不存在: %d", userID)
		}
		return nil, err
	}

	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	grant := &TestPackGrant{Product: product, Count: count}
	if product == model.ServiceSMS || product == model.ProductSMSMarketing {
		up := &model.SmsUserResourcePack{
			UserID:         userID,
			TotalCount:     count,
			RemainingCount: count,
			Product:        product, // 类型快照：与模板类型匹配消费
			Price:          0,
			Status:         1,
		}
		if err := s.smsResourcePackRepo.CreateUserPackTx(tx, up); err != nil {
			return nil, err
		}
		grant.UserPackID = up.ID
	} else {
		up := &model.UserResourcePack{
			UserID:         userID,
			TotalCount:     count,
			RemainingCount: count,
			Product:        product, // 子产品快照：有源/无源各自独立抵扣，互不通用
			Price:          0,
			Status:         1,
		}
		if err := s.resourcePackRepo.CreateUserPackTx(tx, up); err != nil {
			return nil, err
		}
		grant.UserPackID = up.ID
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	audit.Log("pack_grant_test",
		audit.KV("admin_id", adminID),
		audit.KV("user_id", userID),
		audit.KV("product", product),
		audit.KV("count", count),
		audit.KV("user_pack_id", grant.UserPackID))
	return grant, nil
}

// CheckSmsQuota 发送前额度校验（不扣费）：packProduct 类型的短信资源包剩余条数够则放行，否则要求余额足够支付「条数 × 单价」。
// 短信发送在成功后扣费，此校验用于保持「额度不足则不发送」，避免发出后无法计费。
func (s *BalanceService) CheckSmsQuota(userID int64, count int, unitPrice float64, packProduct string) error {
	if count <= 0 {
		count = 1
	}
	pack, err := s.smsResourcePackRepo.GetUserActivePack(userID, packProduct)
	if err != nil {
		return fmt.Errorf("查询短信资源包失败: %w", err)
	}
	if pack != nil && pack.RemainingCount >= count {
		return nil
	}
	balance, err := s.userRepo.GetBalance(userID)
	if err != nil {
		return fmt.Errorf("查询余额失败: %w", err)
	}
	if balance+1e-6 < unitPrice*float64(count) {
		return fmt.Errorf("短信资源包不足且余额不足，请先充值或购买短信资源包")
	}
	return nil
}

// DeductSmsFee 短信扣费：先扣短信库中 packProduct 类型的短信资源包条数（按 count 条），未命中则扣余额。
// 与 DeductProductFee 一致，仅资源包源为短信库独立表（按验证码/通知与营销两类隔离匹配）。
// 返回实际计费方式（0-资源包 1-余额）与命中的资源包 ID（余额计费时为 0）。
func (s *BalanceService) DeductSmsFee(userID int64, amount float64, count, orderID int64, remark, refType, packProduct string) (int, int64, error) {
	pack, err := s.smsResourcePackRepo.GetUserActivePack(userID, packProduct)
	if err != nil {
		return 0, 0, fmt.Errorf("查询短信资源包失败: %w", err)
	}
	if pack != nil {
		ok, derr := s.smsResourcePackRepo.DeductUserPackCount(pack.ID, userID, int(count))
		if derr != nil {
			return 0, 0, derr
		}
		if ok {
			// 资源包扣量不写统一账单（bill 只记资金变动），扣减条数记录在业务表（sms_send_record.pack_count）
			return model.PayTypePack, pack.ID, nil
		}
	}
	if err := s.DeductBalance(userID, amount, orderID, remark, productOf(model.ServiceSMS), model.ServiceSMS, refType); err != nil {
		return 0, 0, err
	}
	return model.PayTypeBalance, 0, nil
}

// RefundSmsCharge 退还短信扣费（回执失败等场景）：资源包按扣减条数退回原包（未记包 ID 的历史记录退到用户最近一条包），
// 余额支付按金额原路退还并写退款账单。
func (s *BalanceService) RefundSmsCharge(userID int64, payType int, packID int64, packCount int, amount float64, orderID int64) error {
	if payType == model.PayTypePack && packCount > 0 {
		if packID > 0 {
			return s.smsResourcePackRepo.RefundUserPackCount(packID, packCount)
		}
		return s.smsResourcePackRepo.RefundUserPackCountByUser(userID, packCount)
	}
	if amount > 0 {
		return s.RefundBalance(userID, amount, orderID, "短信回执失败退款", "sms_send_record")
	}
	return nil
}

// RefundBalance 退还余额（refType 标识原扣费绑定的业务表类型，用于账单关联）
func (s *BalanceService) RefundBalance(userID int64, amount float64, orderID int64, remark, refType string) error {
	// 开启事务
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 在事务中锁定并查询用户当前余额
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return err
	}

	// 增加余额
	newBalance := balance + amount
	if err := s.userRepo.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}

	// 记录统一账单（退款）
	bill := &model.Bill{
		UserID:        userID,
		Product:       "",
		Service:       "refund",
		BillType:      model.BillTypeRefund,
		SpendType:     model.SpendTypeRefund,
		PayType:       model.PayTypeBalance,
		Amount:        amount,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       refType,
		RefID:         orderID,
		Remark:        remark,
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return err
	}

	return tx.Commit()
}

// CreditBalanceTx 站内资金划转入账（在调用方事务内执行）：增加用户余额并写统一账单。
// 用于推广收益提现到余额等内部划转：资金不经外部支付渠道，账单计费方式记为余额。
func (s *BalanceService) CreditBalanceTx(tx *sql.Tx, userID int64, amount float64, remark, refType string, refID int64) error {
	if amount <= 0 {
		return nil
	}
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return err
	}
	newBalance := roundAmount(balance + amount)
	if err := s.userRepo.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}
	return s.writeBillTx(tx, &model.Bill{
		UserID:        userID,
		Product:       "",
		Service:       "aff_withdraw",
		BillType:      model.BillTypeRecharge,
		SpendType:     model.SpendTypeRecharge,
		PayType:       model.PayTypeBalance,
		Amount:        amount,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       refType,
		RefID:         refID,
		Remark:        remark,
	})
}

// RechargeBalance 充值到账（channel 为支付渠道，决定账单计费方式）
func (s *BalanceService) RechargeBalance(userID int64, amount float64, orderID int64, channel string) error {
	// 开启事务
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 在事务中锁定并查询用户当前余额
	balance, err := s.userRepo.GetBalanceForUpdateTx(tx, userID)
	if err != nil {
		return err
	}

	// 增加余额
	newBalance := balance + amount
	if err := s.userRepo.UpdateUserBalanceTx(tx, userID, newBalance); err != nil {
		return err
	}

	// 记录统一账单（充值入账）
	bill := &model.Bill{
		UserID:        userID,
		Product:       "",
		Service:       "recharge",
		BillType:      model.BillTypeRecharge,
		SpendType:     model.SpendTypeRecharge,
		PayType:       model.PayTypeOfChannel(channel),
		Amount:        amount,
		BalanceBefore: balance,
		BalanceAfter:  newBalance,
		RefType:       "payment_order",
		RefID:         orderID,
		PayOrderID:    orderID,
		Remark:        "充值到账",
	}
	if err := s.writeBillTx(tx, bill); err != nil {
		return err
	}

	return tx.Commit()
}

// DeductProductFee 按子产品扣费（先扣对应 product 的未耗尽资源包，再扣余额），返回实际计费方式：
//   - 命中资源包：扣减资源包次数，记录消费流水与统一账单（spend_type=pack_consume，余额不变）
//   - 未命中资源包：按金额扣除余额（含余额流水与统一账单 spend_type=balance）
//
// 用于子产品（如短信 sms、fv_auth/fv_self 等）的按产品计费；
// 不同子产品资源包互不通用，须与本单所属 product 精确匹配。
// refType/orderID 用于账单绑定业务表（如 sms_send_record）。
// 返回实际计费方式（0-资源包 1-余额）与命中的资源包 ID（余额计费时为 0）。
func (s *BalanceService) DeductProductFee(userID int64, product, service string, amount float64, orderID int64, remark, refType string) (int, int64, error) {
	pack, err := s.resourcePackRepo.GetUserActivePack(userID, product)
	if err != nil {
		return 0, 0, fmt.Errorf("查询资源包失败: %w", err)
	}
	if pack != nil {
		ok, derr := s.resourcePackRepo.DeductUserPackCount(pack.ID, userID)
		if derr != nil {
			return 0, 0, derr
		}
		if ok {
			// 资源包抵扣不写统一账单（bill 只记资金变动），扣减次数记录在业务表（auth_record.pack_count）
			return model.PayTypePack, pack.ID, nil
		}
		// 资源包并发耗尽，回退余额扣费
	}
	if err := s.DeductBalance(userID, amount, orderID, remark, productOf(product), service, refType); err != nil {
		return 0, 0, err
	}
	return model.PayTypeBalance, 0, nil
}

// RefundProductFee 退还按产品扣费：资源包退次数，余额原路退还（上游调用失败时调用）
func (s *BalanceService) RefundProductFee(userID int64, amount float64, payType, packID, orderID int64, remark, refType string) error {
	if payType == model.PayTypePack && packID > 0 {
		return s.resourcePackRepo.RefundUserPackCount(packID)
	}
	return s.RefundBalance(userID, amount, orderID, remark, refType)
}

// DailyReconcile 日终对账（每日 3:30）：对比昨日「渠道已支付流水」与「本地账单入账」总额，
// 差额超过容差（0.01 元）视为异常，列出未入账的支付单并输出 error 级告警。
// 正常时输出 info 级汇总。渠道已支付 = payment_order status=1 且 paid_at 在昨日；
// 本地入账 = bill 中绑定支付单（pay_order_id>0）且 created_at 在昨日的金额合计。
func (s *BalanceService) DailyReconcile() {
	date := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	var channelCount int64
	var channelAmount, billAmount float64
	if err := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(amount), 0) FROM `+model.SysDB+`.payment_order WHERE status = 1 AND DATE(paid_at) = ?`,
		date,
	).Scan(&channelCount, &channelAmount); err != nil {
		log.Printf("日终对账失败：查询渠道已支付流水出错 [date=%s]: %v", date, err)
		return
	}
	if err := s.db.QueryRow(
		`SELECT COALESCE(SUM(amount), 0) FROM `+model.SysDB+`.bill WHERE pay_order_id > 0 AND DATE(created_at) = ?`,
		date,
	).Scan(&billAmount); err != nil {
		log.Printf("日终对账失败：查询本地账单出错 [date=%s]: %v", date, err)
		return
	}

	if math.Abs(channelAmount-billAmount) <= 0.01 {
		log.Printf("日终对账一致 [date=%s, 渠道已支付 %d 笔 %.2f 元，本地入账 %.2f 元]", date, channelCount, channelAmount, billAmount)
		return
	}

	// 差异：列出渠道已支付但本地无账单入账的支付单（可能未写 bill / 已关单却到账）
	rows, err := s.db.Query(
		`SELECT pay_order_no, user_id, amount, paid_at FROM `+model.SysDB+`.payment_order
			WHERE status = 1 AND DATE(paid_at) = ? AND id NOT IN (
				SELECT pay_order_id FROM `+model.SysDB+`.bill WHERE pay_order_id > 0 AND DATE(created_at) = ?
			)`,
		date, date,
	)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var no string
			var uid int64
			var amt float64
			var paidAt time.Time
			if err := rows.Scan(&no, &uid, &amt, &paidAt); err == nil {
				log.Printf("日终对账异常：渠道已支付但本地无账单 [date=%s, pay_order_no=%s, user_id=%d, amount=%.2f, paid_at=%s]", date, no, uid, amt, paidAt.Format("2006-01-02 15:04:05"))
			}
		}
	}
	log.Printf("日终对账差异告警 [date=%s, 渠道已支付 %d 笔 %.2f 元，本地入账 %.2f 元，差额 %.2f 元]", date, channelCount, channelAmount, billAmount, channelAmount-billAmount)
}

// CheckBusinessAlerts 业务告警巡检（每 30 分钟）：
//  1. 认证记录长期卡在待认证/认证中（超过 2 小时未终态）——输出 error 告警；
//  2. 待重推通知堆积（fail_times>=5 或超过 1 小时仍未成功）——输出 error 告警。
//
// 告警仅写 error 日志（docker logs 可见），不落库。
func (s *BalanceService) CheckBusinessAlerts() {
	// 1. FV 认证记录长期 pending
	var pendingRecords int64
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM ` + model.FvDB + `.auth_record WHERE status IN (0, 1) AND created_at < NOW() - INTERVAL 2 HOUR`,
	).Scan(&pendingRecords); err != nil {
		log.Printf("业务告警巡检失败：查询 pending 认证记录出错: %v", err)
	} else if pendingRecords > 0 {
		log.Printf("业务告警：%d 笔认证记录超过 2 小时仍处于待认证/认证中，请检查上游回调与同步任务", pendingRecords)
	}

	// 2. 待重推通知堆积
	var stuckNotify int64
	if err := s.db.QueryRow(
		`SELECT COUNT(*) FROM ` + model.SysDB + `.notify_record
			WHERE status = 0 AND (fail_times >= 5 OR created_at < NOW() - INTERVAL 1 HOUR)`,
	).Scan(&stuckNotify); err != nil {
		log.Printf("业务告警巡检失败：查询待重推通知出错: %v", err)
	} else if stuckNotify > 0 {
		log.Printf("业务告警：%d 条下游通知重试受阻（连续失败或积压超过 1 小时），请检查下游服务", stuckNotify)
	}
}
