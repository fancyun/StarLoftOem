package repository

import (
	"database/sql"
	"fmt"

	"oemrpa/internal/model"
)

// PromotionFinanceRepository 推广提成与提现存取（表均在系统库 oem_sys）。
// 改造为纯提成模式后不再有「进货余额 / 冻结 / 可用」三口径账户：
// 可提现收益由提成流水 user_commission 与提现申请 aff_withdraw 实时汇总得出。
type PromotionFinanceRepository struct {
	db *sql.DB
}

func NewPromotionFinanceRepository(db *sql.DB) *PromotionFinanceRepository {
	return &PromotionFinanceRepository{db: db}
}

// ---------- 提成汇总 ----------

// CommissionSummary 推广收益汇总
type CommissionSummary struct {
	Total     float64 // 累计提成（提成流水合计）
	Withdrawn float64 // 已占用提现额（待审核 / 已通过 / 已完成）
	Available float64 // 可提现 = Total − Withdrawn
}

// SumCommission 汇总某推广方的累计提成与已占用提现额
func (r *PromotionFinanceRepository) SumCommission(referrerType string, referrerID int64) (*CommissionSummary, error) {
	return sumCommission(r.db, referrerType, referrerID)
}

// SumCommissionTx 在事务内汇总（提现申请前校验可提现额度）
func (r *PromotionFinanceRepository) SumCommissionTx(tx *sql.Tx, referrerType string, referrerID int64) (*CommissionSummary, error) {
	return sumCommission(tx, referrerType, referrerID)
}

// SumStaffCommissionInRange 汇总某员工销售在指定时间区间内的提成净额（销售报告按月查看；留空表示累计）
// startDate / endDate 为 "YYYY-MM-DD"（左闭右开，endDate 传次月 1 日；留空表示不限）。
func (r *PromotionFinanceRepository) SumStaffCommissionInRange(referrerType string, referrerID int64, startDate, endDate string) (float64, error) {
	return r.sumCommissionInRange(model.TableStaffCommission, referrerType, referrerID, startDate, endDate)
}

// sumCommissionInRange 提成区间汇总实现：table 区分用户型推广与员工销售两张表
func (r *PromotionFinanceRepository) sumCommissionInRange(table string, referrerType string, referrerID int64, startDate, endDate string) (float64, error) {
	where := ` WHERE referrer_type = ? AND referrer_id = ?`
	args := []interface{}{referrerType, referrerID}
	if startDate != "" {
		where += ` AND created_at >= ?`
		args = append(args, startDate)
	}
	if endDate != "" {
		where += ` AND created_at < ?`
		args = append(args, endDate)
	}
	var sum float64
	err := r.db.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM `+table+where, args...).Scan(&sum)
	return sum, err
}

// rowQuerier 同时兼容 *sql.DB 与 *sql.Tx
type rowQuerier interface {
	QueryRow(query string, args ...interface{}) *sql.Row
}

// LockCommissionOwnerTx 锁定推广方（用户）行，使同一账号的提现申请串行，避免并发超额申请
func (r *PromotionFinanceRepository) LockCommissionOwnerTx(tx *sql.Tx, userID int64) error {
	var got int64
	err := tx.QueryRow(`SELECT id FROM `+model.SysDB+`.user WHERE id = ? FOR UPDATE`, userID).Scan(&got)
	if err == sql.ErrNoRows {
		return fmt.Errorf("用户不存在")
	}
	return err
}

// sumCommission 汇总实现：累计提成（用户型推广流水）、已占用提现额、可提现。
// 员工销售提成存放于 staff_commission 且不参与提现，故不在此口径内（见 SumStaffCommissionInRange）。
func sumCommission(q rowQuerier, referrerType string, referrerID int64) (*CommissionSummary, error) {
	s := &CommissionSummary{}
	if err := q.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM `+model.TableUserCommission+` WHERE referrer_type = ? AND referrer_id = ?`,
		referrerType, referrerID).Scan(&s.Total); err != nil {
		return nil, err
	}
	if err := q.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM `+model.SysDB+`.aff_withdraw
		WHERE referrer_type = ? AND referrer_id = ? AND status IN (?, ?, ?)`,
		referrerType, referrerID, model.PromotionWithdrawPending, model.PromotionWithdrawApproved, model.PromotionWithdrawPaid).Scan(&s.Withdrawn); err != nil {
		return nil, err
	}
	s.Available = s.Total - s.Withdrawn
	return s, nil
}

// ---------- 提现 ----------

const promotionWithdrawColumns = `id, referrer_type, referrer_id, channel, amount, fee_rate, fee, actual_amount, status, payee_info, reject_reason,
	admin_id, reviewed_at, paid_at, created_at, updated_at`

func scanPromotionWithdraw(row interface{ Scan(...interface{}) error }) (*model.PromotionWithdraw, error) {
	w := &model.PromotionWithdraw{}
	err := row.Scan(&w.ID, &w.ReferrerType, &w.ReferrerID, &w.Channel, &w.Amount, &w.FeeRate, &w.Fee, &w.ActualAmount, &w.Status, &w.PayeeInfo,
		&w.RejectReason, &w.AdminID, &w.ReviewedAt, &w.PaidAt, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

// CreateWithdrawTx 在事务中写入提现申请
func (r *PromotionFinanceRepository) CreateWithdrawTx(tx *sql.Tx, w *model.PromotionWithdraw) error {
	res, err := tx.Exec(`INSERT INTO `+model.SysDB+`.aff_withdraw
		(referrer_type, referrer_id, channel, amount, fee_rate, fee, actual_amount, status, payee_info, paid_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW(), NOW())`,
		w.ReferrerType, w.ReferrerID, w.Channel, w.Amount, w.FeeRate, w.Fee, w.ActualAmount, w.Status, w.PayeeInfo, w.PaidAt)
	if err != nil {
		return err
	}
	if id, err := res.LastInsertId(); err == nil {
		w.ID = id
	}
	return nil
}

// GetWithdraw 按 ID 查询提现申请
func (r *PromotionFinanceRepository) GetWithdraw(id int64) (*model.PromotionWithdraw, error) {
	w, err := scanPromotionWithdraw(r.db.QueryRow(`SELECT `+promotionWithdrawColumns+` FROM `+model.SysDB+`.aff_withdraw WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return w, err
}

// GetWithdrawForUpdateTx 在事务中锁定并返回提现申请
func (r *PromotionFinanceRepository) GetWithdrawForUpdateTx(tx *sql.Tx, id int64) (*model.PromotionWithdraw, error) {
	return scanPromotionWithdraw(tx.QueryRow(`SELECT `+promotionWithdrawColumns+` FROM `+model.SysDB+`.aff_withdraw WHERE id = ? FOR UPDATE`, id))
}

// UpdateWithdrawTx 更新提现申请状态与审核信息
func (r *PromotionFinanceRepository) UpdateWithdrawTx(tx *sql.Tx, w *model.PromotionWithdraw) error {
	_, err := tx.Exec(`UPDATE `+model.SysDB+`.aff_withdraw
		SET status = ?, reject_reason = ?, admin_id = ?, reviewed_at = ?, paid_at = ?, updated_at = NOW()
		WHERE id = ?`,
		w.Status, w.RejectReason, w.AdminID, w.ReviewedAt, w.PaidAt, w.ID)
	return err
}

// CountPendingWithdraw 统计某推广方待审核的提现申请数
func (r *PromotionFinanceRepository) CountPendingWithdraw(referrerType string, referrerID int64) (int64, error) {
	var n int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.aff_withdraw WHERE referrer_type = ? AND referrer_id = ? AND status = ?`,
		referrerType, referrerID, model.PromotionWithdrawPending).Scan(&n)
	return n, err
}

// ListWithdraws 分页查询提现申请：referrerType 为空表示不限（后台全量），status<0 表示不限状态
func (r *PromotionFinanceRepository) ListWithdraws(referrerType string, referrerID int64, status, page, pageSize int) ([]*model.PromotionWithdraw, int64, error) {
	where := ` WHERE 1 = 1`
	args := []interface{}{}
	if referrerType != "" {
		where += ` AND referrer_type = ? AND referrer_id = ?`
		args = append(args, referrerType, referrerID)
	}
	if status >= 0 {
		where += ` AND status = ?`
		args = append(args, status)
	}

	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.aff_withdraw`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT ` + promotionWithdrawColumns + ` FROM ` + model.SysDB + `.aff_withdraw` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*model.PromotionWithdraw, 0)
	for rows.Next() {
		w, err := scanPromotionWithdraw(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, w)
	}
	return items, total, rows.Err()
}
