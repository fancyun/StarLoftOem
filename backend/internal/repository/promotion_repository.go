package repository

import (
	"database/sql"
	"fmt"
	"time"

	"oemrpa/internal/model"
)

// 推广分佣相关表：价格覆盖（price_override）按产品分库（系统库承载账户实名两档，
// 人脸核验与短信的覆盖价在各自产品库，表名见 model.PriceOverrideTableByService）；
// 提成与提现（user_commission / staff_commission / aff_withdraw）均在系统库 oem_sys，
// 按 referrer_type + referrer_id 归属（user→user.id，staff→admin_user.id），不再有推广商主体表。
type PromotionRepository struct {
	db *sql.DB
}

func NewPromotionRepository(db *sql.DB) *PromotionRepository {
	return &PromotionRepository{db: db}
}

// ---------- 价格覆盖 ----------

// checkPriceOverrideTable 动态表名白名单校验（表名来自 model，禁止外部字符串直接拼接）
func checkPriceOverrideTable(table string) error {
	if !model.IsValidPriceOverrideTable(table) {
		return fmt.Errorf("非法的价格覆盖表: %s", table)
	}
	return nil
}

// GetPriceOverride 查询单条价格覆盖，第二个返回值为是否存在；table 为全限定表名
func (r *PromotionRepository) GetPriceOverride(table, scopeType string, scopeID int64, priceType, target string) (float64, bool, error) {
	if err := checkPriceOverrideTable(table); err != nil {
		return 0, false, err
	}
	var price float64
	err := r.db.QueryRow(`SELECT price FROM `+table+`
		WHERE scope_type = ? AND scope_id = ? AND price_type = ? AND target = ?`,
		scopeType, scopeID, priceType, target).Scan(&price)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return price, true, nil
}

// ListPriceOverrides 列出某作用域下指定表的全部价格覆盖
func (r *PromotionRepository) ListPriceOverrides(table, scopeType string, scopeID int64) ([]*model.PriceOverride, error) {
	if err := checkPriceOverrideTable(table); err != nil {
		return nil, err
	}
	rows, err := r.db.Query(`SELECT id, scope_type, scope_id, price_type, target, price, created_at, updated_at
		FROM `+table+` WHERE scope_type = ? AND scope_id = ? ORDER BY price_type, target`,
		scopeType, scopeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*model.PriceOverride
	for rows.Next() {
		p := &model.PriceOverride{}
		if err := rows.Scan(&p.ID, &p.ScopeType, &p.ScopeID, &p.PriceType, &p.Target, &p.Price, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, p)
	}
	return items, rows.Err()
}

// UpsertPriceOverride 写入或更新价格覆盖
func (r *PromotionRepository) UpsertPriceOverride(table, scopeType string, scopeID int64, priceType, target string, price float64) error {
	if err := checkPriceOverrideTable(table); err != nil {
		return err
	}
	_, err := r.db.Exec(`INSERT INTO `+table+` (scope_type, scope_id, price_type, target, price, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE price = VALUES(price), updated_at = NOW()`,
		scopeType, scopeID, priceType, target, price)
	return err
}

// DeletePriceOverride 删除指定价格覆盖（删除后回落平台价）
func (r *PromotionRepository) DeletePriceOverride(table, scopeType string, scopeID int64, priceType, target string) error {
	if err := checkPriceOverrideTable(table); err != nil {
		return err
	}
	_, err := r.db.Exec(`DELETE FROM `+table+`
		WHERE scope_type = ? AND scope_id = ? AND price_type = ? AND target = ?`,
		scopeType, scopeID, priceType, target)
	return err
}

// ---------- 结算流水 ----------

// CreateSettlementTx 在事务中写入提成流水。
// table 由 model.SettlementTable(referrerType) 给出（用户型推广/员工销售分表存放）。
func (r *PromotionRepository) CreateSettlementTx(table string, tx *sql.Tx, s *model.UserCommission) error {
	_, err := tx.Exec(`INSERT INTO `+table+` (referrer_type, referrer_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())`,
		s.ReferrerType, s.ReferrerID, s.UserID, s.Amount, s.BizType, s.RefType, s.RefID, s.Remark)
	return err
}

// ListSettlements 分页查询用户型推广的提成流水（不限时间）
func (r *PromotionRepository) ListSettlements(referrerType string, referrerID int64, page, pageSize int) ([]*model.UserCommission, int64, error) {
	return r.listSettlementsByRange(model.TableUserCommission, referrerType, referrerID, "", "", page, pageSize)
}

// ListStaffCommissions 分页查询员工销售的提成流水（不限时间）
func (r *PromotionRepository) ListStaffCommissions(referrerType string, referrerID int64, page, pageSize int) ([]*model.UserCommission, int64, error) {
	return r.listSettlementsByRange(model.TableStaffCommission, referrerType, referrerID, "", "", page, pageSize)
}

// ListStaffCommissionsByRange 分页查询员工销售的提成流水，可按时间区间过滤（销售报告按月查看）
func (r *PromotionRepository) ListStaffCommissionsByRange(referrerType string, referrerID int64, startDate, endDate string, page, pageSize int) ([]*model.UserCommission, int64, error) {
	return r.listSettlementsByRange(model.TableStaffCommission, referrerType, referrerID, startDate, endDate, page, pageSize)
}

// listSettlementsByRange 提成流水分页实现：table 区分用户型推广与员工销售两张表。
// startDate / endDate 为 "YYYY-MM-DD"（左闭右开，endDate 传次月 1 日；留空表示不限）。
func (r *PromotionRepository) listSettlementsByRange(table string, referrerType string, referrerID int64, startDate, endDate string, page, pageSize int) ([]*model.UserCommission, int64, error) {
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

	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+table+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(`SELECT id, referrer_type, referrer_id, user_id, amount, biz_type, ref_type, ref_id, remark, created_at
		FROM `+table+where+` ORDER BY id DESC LIMIT ? OFFSET ?`,
		append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []*model.UserCommission
	for rows.Next() {
		s := &model.UserCommission{}
		if err := rows.Scan(&s.ID, &s.ReferrerType, &s.ReferrerID, &s.UserID, &s.Amount, &s.BizType, &s.RefType, &s.RefID, &s.Remark, &s.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, s)
	}
	return items, total, rows.Err()
}

// SumCommissionByRef 汇总某业务单据在指定提成表中的提成净额（已计提 − 已冲回）。
// 供退款按原计提额冲回，避免两个问题：
//  1. 提成比例被后台事后调整时，按新比例重算会与实际计提不符（少冲=推广方白拿，多冲=推广方倒亏）；
//  2. 同一单据被重复退款时超额冲回 —— 取净额可保证冲回总额不超过实际计提额。
func (r *PromotionRepository) SumCommissionByRef(table string, userID int64, refType string, refID int64) (float64, error) {
	var sum float64
	err := r.db.QueryRow(`SELECT COALESCE(SUM(amount), 0) FROM `+table+`
		WHERE user_id = ? AND ref_type = ? AND ref_id = ?`,
		userID, refType, refID).Scan(&sum)
	return sum, err
}

// ---------- 提成记录（平台后台） ----------

// CommissionRecord 提成记录（user_commission 与 staff_commission 两表 UNION ALL 的视图行）
type CommissionRecord struct {
	ID           int64     `json:"id"`
	ReferrerType string    `json:"referrer_type"`
	ReferrerID   int64     `json:"referrer_id"`
	ReferrerName string    `json:"referrer_name"` // 推广方名称（用户/员工）
	UserID       int64     `json:"user_id"`
	Username     string    `json:"username"` // 来源用户
	Amount       float64   `json:"amount"`
	BizType      string    `json:"biz_type"`
	RefType      string    `json:"ref_type"`
	RefID        int64     `json:"ref_id"`
	Remark       string    `json:"remark"`
	CreatedAt    time.Time `json:"created_at"`
}

// ListAllCommissions 分页查询全部提成记录：两表 UNION ALL 后按创建时间倒序。
// referrerType / bizType / keyword 留空表示不限；startDate / endDate 为 "YYYY-MM-DD"（含边界）。
func (r *PromotionRepository) ListAllCommissions(referrerType, bizType, keyword, startDate, endDate string, page, pageSize int) ([]*CommissionRecord, int64, error) {
	union := `SELECT c.id, c.referrer_type, c.referrer_id,
			COALESCE(u.username, '') AS referrer_name,
			c.user_id, COALESCE(su.username, '') AS username,
			c.amount, c.biz_type, c.ref_type, c.ref_id, c.remark, c.created_at
		FROM ` + model.TableUserCommission + ` c
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = c.referrer_id
		LEFT JOIN ` + model.SysDB + `.user su ON su.id = c.user_id
		UNION ALL
		SELECT c.id, c.referrer_type, c.referrer_id,
			COALESCE(a.username, '') AS referrer_name,
			c.user_id, COALESCE(su.username, '') AS username,
			c.amount, c.biz_type, c.ref_type, c.ref_id, c.remark, c.created_at
		FROM ` + model.TableStaffCommission + ` c
		LEFT JOIN ` + model.SysDB + `.admin_user a ON a.id = c.referrer_id
		LEFT JOIN ` + model.SysDB + `.user su ON su.id = c.user_id`

	where := ` WHERE 1 = 1`
	args := []interface{}{}
	if referrerType != "" {
		where += ` AND t.referrer_type = ?`
		args = append(args, referrerType)
	}
	if bizType != "" {
		where += ` AND t.biz_type = ?`
		args = append(args, bizType)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		where += ` AND (t.referrer_name LIKE ? OR t.username LIKE ?)`
		args = append(args, like, like)
	}
	if startDate != "" {
		where += ` AND t.created_at >= ?`
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += ` AND t.created_at <= ?`
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM (`+union+`) t`+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	rows, err := r.db.Query(`SELECT t.* FROM (`+union+`) t`+where+` ORDER BY t.created_at DESC, t.id DESC LIMIT ? OFFSET ?`,
		append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	items := make([]*CommissionRecord, 0)
	for rows.Next() {
		c := &CommissionRecord{}
		if err := rows.Scan(&c.ID, &c.ReferrerType, &c.ReferrerID, &c.ReferrerName, &c.UserID, &c.Username,
			&c.Amount, &c.BizType, &c.RefType, &c.RefID, &c.Remark, &c.CreatedAt); err != nil {
			return nil, 0, err
		}
		items = append(items, c)
	}
	return items, total, rows.Err()
}
