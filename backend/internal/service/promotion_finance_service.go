package service

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"oemrpa/internal/model"
	"oemrpa/internal/repository"
)

// 推广提现错误
var (
	// ErrPromotionWithdrawInsufficient 提现金额超过可提现收益
	ErrPromotionWithdrawInsufficient = errors.New("提现金额超过可提现收益")
)

// PromotionFinanceService 推广收益与提现。
// 改造为纯提成模式后不再持有独立财务账户：可提现收益由提成流水（user_commission）
// 与提现申请（aff_withdraw）实时汇总得出；被驳回或未通过的申请自动释放占用额度。
type PromotionFinanceService struct {
	db          *sql.DB
	financeRepo *repository.PromotionFinanceRepository
	// creditBalanceTx 站内划转入账（推广收益提现到余额），由余额服务在装配时注入。
	// 传入调用方事务，保证「写提现记录」与「增加余额」在同一事务内原子完成。
	creditBalanceTx func(tx *sql.Tx, userID int64, amount float64, remark, refType string, refID int64) error
}

// SetBalanceCreditor 注入站内划转入账实现（在路由装配阶段调用）
func (s *PromotionFinanceService) SetBalanceCreditor(fn func(tx *sql.Tx, userID int64, amount float64, remark, refType string, refID int64) error) {
	s.creditBalanceTx = fn
}

func NewPromotionFinanceService(db *sql.DB, financeRepo *repository.PromotionFinanceRepository) *PromotionFinanceService {
	return &PromotionFinanceService{db: db, financeRepo: financeRepo}
}

// ---------- 收益视图 ----------

// AffFinanceView 推广收益视图（控制台「推广」页使用）
type AffFinanceView struct {
	TotalCommission float64 `json:"total_commission"` // 累计提成
	Withdrawn       float64 `json:"withdrawn"`        // 已占用提现额（待审核 / 已通过 / 已完成）
	Available       float64 `json:"available"`        // 可提现收益 = 累计提成 − 已占用提现额
	PendingWithdraw int64   `json:"pending_withdraw"` // 待审核提现笔数
}

// CommissionView 组装推广收益视图（用户型推广：referrerType=user, referrerID=user.id）
func (s *PromotionFinanceService) CommissionView(referrerType string, referrerID int64) (*AffFinanceView, error) {
	sum, err := s.financeRepo.SumCommission(referrerType, referrerID)
	if err != nil {
		return nil, err
	}
	pending, err := s.financeRepo.CountPendingWithdraw(referrerType, referrerID)
	if err != nil {
		return nil, err
	}
	return &AffFinanceView{
		TotalCommission: sum.Total,
		Withdrawn:       sum.Withdrawn,
		Available:       sum.Available,
		PendingWithdraw: pending,
	}, nil
}

// StaffCommissionInRange 汇总某员工销售在指定时间区间内的提成净额（留空区间表示累计）：
// 员工销售提成长期有效，不参与提现（无 aff_withdraw 口径），故直接用流水合计。
func (s *PromotionFinanceService) StaffCommissionInRange(referrerType string, referrerID int64, startDate, endDate string) (float64, error) {
	return s.financeRepo.SumStaffCommissionInRange(referrerType, referrerID, startDate, endDate)
}

// ---------- 提现 ----------

// ApplyWithdraw 发起提现（channel 为提现方式，当前仅支持「提现到余额」）。
//
// 提现到余额属站内划转：即时到账、免手续费、无需收款信息 —— 落一条「已完成」记录，
// 并在同一事务内把收益划转到平台余额。
// 提现仅面向用户型推广（userID 即推广收益归属的收款账号）；员工型销售的提成只记录流水、不提供提现。
// 线下渠道（支付宝/微信/银行卡等）后续扩展：届时才需要收款信息并走人工审核
// （保留 ReviewWithdraw / MarkWithdrawPaid 供其复用）。
func (s *PromotionFinanceService) ApplyWithdraw(userID int64, amount float64, channel, payeeInfo string) (*model.PromotionWithdraw, error) {
	if userID <= 0 {
		return nil, fmt.Errorf("推广身份无效")
	}
	if amount <= 0 {
		return nil, fmt.Errorf("提现金额须大于 0")
	}
	channel = strings.TrimSpace(channel)
	if channel == "" {
		channel = model.PromotionWithdrawChannelBalance
	}
	if channel != model.PromotionWithdrawChannelBalance {
		return nil, fmt.Errorf("暂不支持该提现方式")
	}
	if s.creditBalanceTx == nil {
		return nil, fmt.Errorf("提现入账能力未装配")
	}

	referrerType := model.RefTypeReferrerUser
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	// 同账号提现串行化：先锁用户行，再汇总可提现额度，避免并发下的超额申请
	if err := s.financeRepo.LockCommissionOwnerTx(tx, userID); err != nil {
		return nil, err
	}
	sum, err := s.financeRepo.SumCommissionTx(tx, referrerType, userID)
	if err != nil {
		return nil, err
	}
	if sum.Available+1e-6 < amount {
		return nil, ErrPromotionWithdrawInsufficient
	}

	now := time.Now()
	w := &model.PromotionWithdraw{
		ReferrerType: referrerType,
		ReferrerID:   userID,
		Channel:      channel,
		Amount:       amount,
		FeeRate:      0, // 提现到余额免手续费
		Fee:          0,
		ActualAmount: amount,
		Status:       model.PromotionWithdrawPaid, // 站内划转即时到账，无需人工审核
		PayeeInfo:    "",
		PaidAt:       &now,
	}
	if err := s.financeRepo.CreateWithdrawTx(tx, w); err != nil {
		return nil, err
	}
	// 同步划转到平台余额（收款方为推广者本人的用户账号）：与写提现记录同一事务，任一步失败整体回滚
	if err := s.creditBalanceTx(tx, userID, amount, "推广收益提现到余额", "aff_withdraw", w.ID); err != nil {
		return nil, err
	}
	return w, tx.Commit()
}

// ReviewWithdraw 审核提现申请：通过则进入待打款；驳回后该笔占用额度随状态自动释放（无需回滚账户）
func (s *PromotionFinanceService) ReviewWithdraw(id int64, approve bool, reason string, adminID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	w, err := s.financeRepo.GetWithdrawForUpdateTx(tx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("提现申请不存在")
	}
	if w.Status != model.PromotionWithdrawPending {
		return fmt.Errorf("该申请已处理，不能重复审核")
	}
	now := time.Now()
	w.AdminID = adminID
	w.ReviewedAt = &now
	if approve {
		w.Status = model.PromotionWithdrawApproved
	} else {
		w.Status = model.PromotionWithdrawRejected
		w.RejectReason = strings.TrimSpace(reason)
	}
	if err := s.financeRepo.UpdateWithdrawTx(tx, w); err != nil {
		return err
	}
	return tx.Commit()
}

// MarkWithdrawPaid 标记提现已线下打款完成
func (s *PromotionFinanceService) MarkWithdrawPaid(id int64, adminID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	w, err := s.financeRepo.GetWithdrawForUpdateTx(tx, id)
	if err != nil {
		return err
	}
	if w == nil {
		return fmt.Errorf("提现申请不存在")
	}
	if w.Status != model.PromotionWithdrawApproved {
		return fmt.Errorf("仅「已通过待打款」的申请可标记完成")
	}
	now := time.Now()
	w.Status = model.PromotionWithdrawPaid
	w.PaidAt = &now
	w.AdminID = adminID
	if err := s.financeRepo.UpdateWithdrawTx(tx, w); err != nil {
		return err
	}
	return tx.Commit()
}

// ListWithdraws 分页查询提现申请（referrerType 为空表示后台全量）
func (s *PromotionFinanceService) ListWithdraws(referrerType string, referrerID int64, status, page, pageSize int) ([]*model.PromotionWithdraw, int64, error) {
	return s.financeRepo.ListWithdraws(referrerType, referrerID, status, page, pageSize)
}
