package repository

import (
	"database/sql"
	"strings"
	"time"

	"oemrpa/internal/model"
)

type BillRepository struct {
	db *sql.DB
}

func NewBillRepository(db *sql.DB) *BillRepository {
	return &BillRepository{db: db}
}

// CreateTx 在事务中创建账单记录
func (r *BillRepository) CreateTx(tx *sql.Tx, bill *model.Bill) error {
	query := `INSERT INTO ` + model.SysDB + `.bill
		(biz_no, user_id, product, service, bill_type, spend_type, pay_type, amount,
		 balance_before, balance_after, ref_type, ref_id, ref_biz_no, pay_order_id, remark, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := tx.Exec(query,
		bill.BizNo,
		bill.UserID,
		bill.Product,
		bill.Service,
		bill.BillType,
		bill.SpendType,
		bill.PayType,
		bill.Amount,
		bill.BalanceBefore,
		bill.BalanceAfter,
		bill.RefType,
		bill.RefID,
		bill.RefBizNo,
		bill.PayOrderID,
		bill.Remark,
		time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	bill.ID = id
	return nil
}

// Create 创建账单记录（非事务，用于独立记账场景）
func (r *BillRepository) Create(bill *model.Bill) error {
	query := `INSERT INTO ` + model.SysDB + `.bill
		(biz_no, user_id, product, service, bill_type, spend_type, pay_type, amount,
		 balance_before, balance_after, ref_type, ref_id, ref_biz_no, pay_order_id, remark, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query,
		bill.BizNo,
		bill.UserID,
		bill.Product,
		bill.Service,
		bill.BillType,
		bill.SpendType,
		bill.PayType,
		bill.Amount,
		bill.BalanceBefore,
		bill.BalanceAfter,
		bill.RefType,
		bill.RefID,
		bill.RefBizNo,
		bill.PayOrderID,
		bill.Remark,
		time.Now(),
	)
	return err
}

// List 查询账单（管理后台：可按 user_id/product/bill_type/spend_type 过滤，分页倒序）
func (r *BillRepository) List(f *BillFilter) ([]*model.Bill, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if f.UserID > 0 {
		where = append(where, "b.user_id = ?")
		args = append(args, f.UserID)
	}
	if f.Product != "" {
		where = append(where, "b.product = ?")
		args = append(args, f.Product)
	}
	if f.BillType > 0 {
		where = append(where, "b.bill_type = ?")
		args = append(args, f.BillType)
	}
	if f.SpendType != "" {
		where = append(where, "b.spend_type = ?")
		args = append(args, f.SpendType)
	}
	cond := strings.Join(where, " AND ")

	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.bill b WHERE `+cond, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	offset := (f.Page - 1) * f.PageSize
	query := `SELECT b.id, b.biz_no, b.user_id, b.product, b.service, b.bill_type, b.spend_type, b.pay_type,
		b.amount, b.balance_before, b.balance_after, b.ref_type, b.ref_id, b.ref_biz_no, b.pay_order_id,
		COALESCE(po.bank_serial_no, ''), b.remark, b.created_at
		FROM ` + model.SysDB + `.bill b
		LEFT JOIN ` + model.SysDB + `.payment_order po ON po.id = b.pay_order_id
		WHERE ` + cond + ` ORDER BY b.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(args, f.PageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	bills := make([]*model.Bill, 0)
	for rows.Next() {
		b := &model.Bill{}
		if err := rows.Scan(
			&b.ID, &b.BizNo, &b.UserID, &b.Product, &b.Service, &b.BillType, &b.SpendType,
			&b.PayType, &b.Amount, &b.BalanceBefore, &b.BalanceAfter, &b.RefType, &b.RefID,
			&b.RefBizNo, &b.PayOrderID, &b.BankSerialNo, &b.Remark, &b.CreatedAt,
		); err != nil {
			return nil, 0, err
		}
		bills = append(bills, b)
	}
	return bills, total, nil
}

// GetUserBills 查询用户账单（分页倒序）
func (r *BillRepository) GetUserBills(userID int64, page, pageSize int) ([]*model.Bill, int64, error) {
	return r.List(&BillFilter{UserID: userID, Page: page, PageSize: pageSize})
}

// Stats 汇总统计：充值总额/消费总额/退款总额（可选按 user_id 过滤）
func (r *BillRepository) Stats(userID int64) (map[string]float64, error) {
	query := `SELECT
		COALESCE(SUM(CASE WHEN bill_type = 1 THEN amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN bill_type = 2 THEN amount ELSE 0 END), 0),
		COALESCE(SUM(CASE WHEN bill_type = 3 THEN amount ELSE 0 END), 0)
		FROM ` + model.SysDB + `.bill`
	args := []interface{}{}
	if userID > 0 {
		query += ` WHERE user_id = ?`
		args = append(args, userID)
	}
	var recharge, consume, refund float64
	if err := r.db.QueryRow(query, args...).Scan(&recharge, &consume, &refund); err != nil {
		return nil, err
	}
	return map[string]float64{
		"totalRecharge": recharge,
		"totalConsume":  consume,
		"totalRefund":   refund,
	}, nil
}

// BillFilter 账单列表过滤条件
type BillFilter struct {
	UserID    int64
	Product   string
	BillType  int
	SpendType string
	Page      int
	PageSize  int
}
