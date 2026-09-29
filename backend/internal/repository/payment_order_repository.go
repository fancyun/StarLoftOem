package repository

import (
	"database/sql"
	"errors"
	"oemrpa/internal/model"
	"time"
)

var (
	ErrPaymentOrderNotFound = errors.New("payment order not found")
)

type PaymentOrderRepository struct {
	db *sql.DB
}

func NewPaymentOrderRepository(db *sql.DB) *PaymentOrderRepository {
	return &PaymentOrderRepository{db: db}
}

// CreateOrder 创建支付订单
func (r *PaymentOrderRepository) CreateOrder(order *model.PaymentOrder) error {
	query := `INSERT INTO ` + model.SysDB + `.payment_order 
		(pay_order_no, user_id, amount, channel, status, 
		expire_time, created_at, updated_at, refund_status, intent, biz_no, balance_amount, stock_reserved, pay_info) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.Exec(query,
		order.PayOrderNo,
		order.UserID,
		order.Amount,
		order.Channel,
		order.Status,
		order.ExpireTime,
		time.Now(),
		time.Now(),
		order.RefundStatus,
		order.Intent,
		order.BizNo,
		order.BalanceAmount,
		order.StockReserved,
		order.PayInfo,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	order.ID = id
	return nil
}

// CreateOrderTx 在事务中创建支付订单（用于购买资源包组合支付的原子落库）
func (r *PaymentOrderRepository) CreateOrderTx(tx *sql.Tx, order *model.PaymentOrder) error {
	query := `INSERT INTO ` + model.SysDB + `.payment_order 
		(pay_order_no, user_id, amount, channel, status, 
		expire_time, created_at, updated_at, refund_status, intent, biz_no, balance_amount, stock_reserved, pay_info) 
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := tx.Exec(query,
		order.PayOrderNo,
		order.UserID,
		order.Amount,
		order.Channel,
		order.Status,
		order.ExpireTime,
		time.Now(),
		time.Now(),
		order.RefundStatus,
		order.Intent,
		order.BizNo,
		order.BalanceAmount,
		order.StockReserved,
		order.PayInfo,
	)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	order.ID = id
	return nil
}

// CreateManualPaymentTx 在事务中创建人工支付记录（后台人工充值：渠道 manual、直接为已支付状态）
func (r *PaymentOrderRepository) CreateManualPaymentTx(tx *sql.Tx, order *model.PaymentOrder) error {
	query := `INSERT INTO ` + model.SysDB + `.payment_order
		(pay_order_no, user_id, amount, channel, channel_trade_no, bank_serial_no, status, paid_at,
		 intent, biz_no, balance_amount, stock_reserved, pay_info, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := tx.Exec(query,
		order.PayOrderNo,
		order.UserID,
		order.Amount,
		order.Channel,
		order.ChannelTradeNo,
		order.BankSerialNo,
		order.Status,
		order.PaidAt,
		order.Intent,
		order.BizNo,
		order.BalanceAmount,
		order.StockReserved,
		order.PayInfo,
		time.Now(),
		time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	order.ID = id
	return nil
}

// SumPaidAmount 按已支付支付单统计金额（where 为追加过滤片段，如 "DATE(paid_at) = ?"），
// 口径与后台「支付记录」一致：status=1 的支付单，涵盖支付宝/微信/人工支付。
func (r *PaymentOrderRepository) SumPaidAmount(where string, args ...interface{}) (float64, error) {
	query := `SELECT COALESCE(SUM(amount), 0) FROM ` + model.SysDB + `.payment_order WHERE status = 1`
	if where != "" {
		query += " AND " + where
	}
	var amount float64
	if err := r.db.QueryRow(query, args...).Scan(&amount); err != nil {
		return 0, err
	}
	return amount, nil
}

// SumDailyOnlineAmount 统计用户当日在线支付（支付宝/微信）占用金额：
// 已支付（按 paid_at 归日）与待支付（按 created_at 归日）订单金额之和，
// 用于「单用户单日在线支付限额」校验，避免大额转账被渠道收取高额手续费。
func (r *PaymentOrderRepository) SumDailyOnlineAmount(userID int64) (float64, error) {
	query := `SELECT COALESCE(SUM(amount), 0) FROM ` + model.SysDB + `.payment_order
		WHERE user_id = ? AND channel IN (?, ?)
		  AND ((status = 1 AND paid_at >= CURDATE()) OR (status = 0 AND created_at >= CURDATE()))`
	var amount float64
	err := r.db.QueryRow(query, userID, model.ChannelAlipay, model.ChannelWechat).Scan(&amount)
	return amount, err
}

// GetDailyPaidAmounts 按天统计已支付金额（支付记录口径，按支付时间归日）
func (r *PaymentOrderRepository) GetDailyPaidAmounts(startDate, endDate string) (map[string]float64, error) {
	query := `SELECT DATE_FORMAT(paid_at, '%Y-%m-%d') AS d, COALESCE(SUM(amount), 0) AS amount
		FROM ` + model.SysDB + `.payment_order
		WHERE status = 1 AND DATE(paid_at) >= ? AND DATE(paid_at) <= ?
		GROUP BY DATE_FORMAT(paid_at, '%Y-%m-%d')`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]float64)
	for rows.Next() {
		var d string
		var amount float64
		if err := rows.Scan(&d, &amount); err != nil {
			return nil, err
		}
		result[d] = amount
	}
	return result, nil
}

// BankSerialExists 校验银行流水单号是否已被人工支付记录占用（人工支付幂等）
func (r *PaymentOrderRepository) BankSerialExists(bankSerialNo string) (bool, error) {
	var n int64
	err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.payment_order WHERE bank_serial_no = ?`, bankSerialNo).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetOrderByID 根据ID查询支付订单
func (r *PaymentOrderRepository) GetOrderByID(orderID int64) (*model.PaymentOrder, error) {
	query := `SELECT id, pay_order_no, user_id, amount, 
		channel, COALESCE(channel_trade_no, ''), status, expire_time, paid_at, created_at, updated_at, 
		refund_status, COALESCE(refund_amount, 0), refunded_at, intent, COALESCE(biz_no, ''), 
		COALESCE(balance_amount, 0), COALESCE(stock_reserved, 0), COALESCE(pay_info, '') 
		FROM ` + model.SysDB + `.payment_order WHERE id = ?`

	order := &model.PaymentOrder{}
	err := r.db.QueryRow(query, orderID).Scan(
		&order.ID,
		&order.PayOrderNo,
		&order.UserID,
		&order.Amount,
		&order.Channel,
		&order.ChannelTradeNo,
		&order.Status,
		&order.ExpireTime,
		&order.PaidAt,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.RefundStatus,
		&order.RefundAmount,
		&order.RefundedAt,
		&order.Intent,
		&order.BizNo,
		&order.BalanceAmount,
		&order.StockReserved,
		&order.PayInfo,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPaymentOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return order, nil
}

// GetOrderByPayOrderNo 根据支付流水号查询订单
func (r *PaymentOrderRepository) GetOrderByPayOrderNo(payOrderNo string) (*model.PaymentOrder, error) {
	query := `SELECT id, pay_order_no, user_id, amount, 
		channel, COALESCE(channel_trade_no, ''), status, expire_time, paid_at, created_at, updated_at, 
		refund_status, COALESCE(refund_amount, 0), refunded_at, intent, COALESCE(biz_no, ''), 
		COALESCE(balance_amount, 0), COALESCE(stock_reserved, 0), COALESCE(pay_info, '') 
		FROM ` + model.SysDB + `.payment_order WHERE pay_order_no = ?`

	order := &model.PaymentOrder{}
	err := r.db.QueryRow(query, payOrderNo).Scan(
		&order.ID,
		&order.PayOrderNo,
		&order.UserID,
		&order.Amount,
		&order.Channel,
		&order.ChannelTradeNo,
		&order.Status,
		&order.ExpireTime,
		&order.PaidAt,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.RefundStatus,
		&order.RefundAmount,
		&order.RefundedAt,
		&order.Intent,
		&order.BizNo,
		&order.BalanceAmount,
		&order.StockReserved,
		&order.PayInfo,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPaymentOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return order, nil
}

// MarkOrderPaidIfPending 仅当订单仍为待支付时更新为已支付（幂等）
// 返回是否发生了状态变更
func (r *PaymentOrderRepository) MarkOrderPaidIfPending(orderID int64, channelTradeNo string) (bool, error) {
	query := `UPDATE ` + model.SysDB + `.payment_order 
		SET status = 1, channel_trade_no = ?, paid_at = ?, updated_at = ? 
		WHERE id = ? AND status = 0`
	result, err := r.db.Exec(query, channelTradeNo, time.Now(), time.Now(), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// MarkOrderPaidIfPendingTx 在事务中标记支付单为已支付（幂等，供资源包等组合支付落地使用）
func (r *PaymentOrderRepository) MarkOrderPaidIfPendingTx(tx *sql.Tx, orderID int64, channelTradeNo string) (bool, error) {
	query := `UPDATE ` + model.SysDB + `.payment_order 
		SET status = 1, channel_trade_no = ?, paid_at = ?, updated_at = ? 
		WHERE id = ? AND status = 0`
	result, err := tx.Exec(query, channelTradeNo, time.Now(), time.Now(), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CloseOrderIfPendingTx 在事务中关闭待支付订单（状态 0→3，幂等，供资源包订单超时关闭使用）
func (r *PaymentOrderRepository) CloseOrderIfPendingTx(tx *sql.Tx, orderID int64) (bool, error) {
	query := `UPDATE ` + model.SysDB + `.payment_order 
		SET status = 3, updated_at = ? 
		WHERE id = ? AND status = 0`
	result, err := tx.Exec(query, time.Now(), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// CloseOrderIfPending 非事务关闭待支付订单（状态 0→3，幂等，供对账单边关闭使用）
func (r *PaymentOrderRepository) CloseOrderIfPending(orderID int64) (bool, error) {
	query := `UPDATE ` + model.SysDB + `.payment_order 
		SET status = 3, updated_at = ? 
		WHERE id = ? AND status = 0`
	result, err := r.db.Exec(query, time.Now(), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetPendingOrdersForReconcile 查询早于指定时间创建、仍待支付的支付订单（用于每日对账）
// olderThan 用于排除刚创建仍在正常支付流程中的订单
func (r *PaymentOrderRepository) GetPendingOrdersForReconcile(olderThan time.Time) ([]*model.PaymentOrder, error) {
	query := `SELECT id, pay_order_no, user_id, amount, 
		channel, COALESCE(channel_trade_no, ''), status, expire_time, paid_at, created_at, updated_at, 
		refund_status, COALESCE(refund_amount, 0), refunded_at, intent, COALESCE(biz_no, ''), 
		COALESCE(balance_amount, 0), COALESCE(stock_reserved, 0), COALESCE(pay_info, '') 
		FROM ` + model.SysDB + `.payment_order 
		WHERE status = 0 AND created_at < ?`

	rows, err := r.db.Query(query, olderThan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]*model.PaymentOrder, 0)
	for rows.Next() {
		order := &model.PaymentOrder{}
		if err := rows.Scan(
			&order.ID,
			&order.PayOrderNo,
			&order.UserID,
			&order.Amount,
			&order.Channel,
			&order.ChannelTradeNo,
			&order.Status,
			&order.ExpireTime,
			&order.PaidAt,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.RefundStatus,
			&order.RefundAmount,
			&order.RefundedAt,
			&order.Intent,
			&order.BizNo,
			&order.BalanceAmount,
			&order.StockReserved,
			&order.PayInfo,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

// GetExpiredPendingOrders 查询已过期且仍待支付的支付订单（充值+资源包，用于定时撤回支付）
func (r *PaymentOrderRepository) GetExpiredPendingOrders(now time.Time) ([]*model.PaymentOrder, error) {
	query := `SELECT id, pay_order_no, user_id, amount, 
		channel, COALESCE(channel_trade_no, ''), status, expire_time, paid_at, created_at, updated_at, 
		refund_status, COALESCE(refund_amount, 0), refunded_at, intent, COALESCE(biz_no, ''), 
		COALESCE(balance_amount, 0), COALESCE(stock_reserved, 0), COALESCE(pay_info, '') 
		FROM ` + model.SysDB + `.payment_order 
		WHERE status = 0 AND expire_time IS NOT NULL AND expire_time < ?`

	rows, err := r.db.Query(query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]*model.PaymentOrder, 0)
	for rows.Next() {
		order := &model.PaymentOrder{}
		if err := rows.Scan(
			&order.ID,
			&order.PayOrderNo,
			&order.UserID,
			&order.Amount,
			&order.Channel,
			&order.ChannelTradeNo,
			&order.Status,
			&order.ExpireTime,
			&order.PaidAt,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.RefundStatus,
			&order.RefundAmount,
			&order.RefundedAt,
			&order.Intent,
			&order.BizNo,
			&order.BalanceAmount,
			&order.StockReserved,
			&order.PayInfo,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

// GetPendingOrder 支付查重：查询用户指定用途下仍未过期（expire_time > now）的待支付单（status=0）。
// amount 大于 0 时同时匹配订单金额（充值单金额固定，用于防止复用不同金额的待支付单）；
// 资源包单金额为外部支付部分（随余额支付比例变化），传 0 按 biz_no 精确匹配即可。
func (r *PaymentOrderRepository) GetPendingOrder(userID int64, intent, bizNo, channel string, amount float64) (*model.PaymentOrder, error) {
	query := `SELECT id, pay_order_no, user_id, amount, 
		channel, COALESCE(channel_trade_no, ''), status, expire_time, paid_at, created_at, updated_at, 
		refund_status, COALESCE(refund_amount, 0), refunded_at, intent, COALESCE(biz_no, ''), 
		COALESCE(balance_amount, 0), COALESCE(stock_reserved, 0), COALESCE(pay_info, '') 
		FROM ` + model.SysDB + `.payment_order 
		WHERE user_id = ? AND intent = ? AND COALESCE(biz_no, '') = ? AND channel = ?
		  AND status = 0 AND expire_time IS NOT NULL AND expire_time > ?`
	args := []interface{}{userID, intent, bizNo, channel, time.Now()}
	if amount > 0 {
		query += ` AND amount = ?`
		args = append(args, amount)
	}
	query += ` ORDER BY id DESC LIMIT 1`

	order := &model.PaymentOrder{}
	err := r.db.QueryRow(query, args...).Scan(
		&order.ID,
		&order.PayOrderNo,
		&order.UserID,
		&order.Amount,
		&order.Channel,
		&order.ChannelTradeNo,
		&order.Status,
		&order.ExpireTime,
		&order.PaidAt,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.RefundStatus,
		&order.RefundAmount,
		&order.RefundedAt,
		&order.Intent,
		&order.BizNo,
		&order.BalanceAmount,
		&order.StockReserved,
		&order.PayInfo,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return order, nil
}

// SettleClosedOrderAsRecharge 将已关闭订单转为充值入账：状态 3→1、改写用途为 recharge、记录渠道交易号（幂等）。
// 用于「订单已关闭但用户仍完成支付」的场景：金额冲入余额，支付记录用途由购买产品修改为充值余额。
// 返回是否发生了状态变更。
func (r *PaymentOrderRepository) SettleClosedOrderAsRecharge(orderID int64, channelTradeNo string) (bool, error) {
	query := `UPDATE ` + model.SysDB + `.payment_order 
		SET status = 1, intent = 'recharge', channel_trade_no = ?, paid_at = ?, updated_at = ? 
		WHERE id = ? AND status = 3`
	result, err := r.db.Exec(query, channelTradeNo, time.Now(), time.Now(), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// SettleClosedOrderAsRechargeTx 事务版：将已关闭订单转为充值入账（与余额入账同一事务，保证资金不丢）。
// 返回是否发生了状态变更。
func (r *PaymentOrderRepository) SettleClosedOrderAsRechargeTx(tx *sql.Tx, orderID int64, channelTradeNo string) (bool, error) {
	query := `UPDATE ` + model.SysDB + `.payment_order 
		SET status = 1, intent = 'recharge', channel_trade_no = ?, paid_at = ?, updated_at = ? 
		WHERE id = ? AND status = 3`
	result, err := tx.Exec(query, channelTradeNo, time.Now(), time.Now(), orderID)
	if err != nil {
		return false, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// GetRefundableOrders 查询用户可退款（提现）的已支付充值订单（status=1、intent=recharge、尚有剩余可退款），
// 按创建时间升序（先退较早充值的订单）。人工支付（channel=manual）无渠道可退，排除在外。
func (r *PaymentOrderRepository) GetRefundableOrders(userID int64) ([]*model.PaymentOrder, error) {
	query := `SELECT id, pay_order_no, user_id, amount, 
		channel, COALESCE(channel_trade_no, ''), status, expire_time, paid_at, created_at, updated_at, 
		refund_status, COALESCE(refund_amount, 0), refunded_at, intent, COALESCE(biz_no, ''), 
		COALESCE(balance_amount, 0), COALESCE(stock_reserved, 0), COALESCE(pay_info, '') 
		FROM ` + model.SysDB + `.payment_order 
		WHERE user_id = ? AND status = 1 AND intent = 'recharge' AND channel IN ('alipay', 'wechat')
		  AND amount > COALESCE(refund_amount, 0)
		ORDER BY created_at ASC, id ASC`

	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	orders := make([]*model.PaymentOrder, 0)
	for rows.Next() {
		order := &model.PaymentOrder{}
		if err := rows.Scan(
			&order.ID,
			&order.PayOrderNo,
			&order.UserID,
			&order.Amount,
			&order.Channel,
			&order.ChannelTradeNo,
			&order.Status,
			&order.ExpireTime,
			&order.PaidAt,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.RefundStatus,
			&order.RefundAmount,
			&order.RefundedAt,
			&order.Intent,
			&order.BizNo,
			&order.BalanceAmount,
			&order.StockReserved,
			&order.PayInfo,
		); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

// GetOrderByIDForUpdateTx 在事务中锁定并查询支付订单（FOR UPDATE，供退款等并发敏感流程使用）
func (r *PaymentOrderRepository) GetOrderByIDForUpdateTx(tx *sql.Tx, orderID int64) (*model.PaymentOrder, error) {
	query := `SELECT id, pay_order_no, user_id, amount, 
		channel, COALESCE(channel_trade_no, ''), status, expire_time, paid_at, created_at, updated_at, 
		refund_status, COALESCE(refund_amount, 0), refunded_at, intent, COALESCE(biz_no, ''), 
		COALESCE(balance_amount, 0), COALESCE(stock_reserved, 0), COALESCE(pay_info, '') 
		FROM ` + model.SysDB + `.payment_order WHERE id = ? FOR UPDATE`

	order := &model.PaymentOrder{}
	err := tx.QueryRow(query, orderID).Scan(
		&order.ID,
		&order.PayOrderNo,
		&order.UserID,
		&order.Amount,
		&order.Channel,
		&order.ChannelTradeNo,
		&order.Status,
		&order.ExpireTime,
		&order.PaidAt,
		&order.CreatedAt,
		&order.UpdatedAt,
		&order.RefundStatus,
		&order.RefundAmount,
		&order.RefundedAt,
		&order.Intent,
		&order.BizNo,
		&order.BalanceAmount,
		&order.StockReserved,
		&order.PayInfo,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPaymentOrderNotFound
	}
	if err != nil {
		return nil, err
	}
	return order, nil
}

// UpdateRefundTx 在事务中更新支付订单退款累计与状态（refund_status：0-未退款 1-部分退款 2-全额退款）
func (r *PaymentOrderRepository) UpdateRefundTx(tx *sql.Tx, orderID int64, refundAmount float64, refundStatus int) error {
	query := `UPDATE ` + model.SysDB + `.payment_order 
		SET refund_amount = ?, refund_status = ?, refunded_at = ?, updated_at = ? 
		WHERE id = ?`
	_, err := tx.Exec(query, refundAmount, refundStatus, time.Now(), time.Now(), orderID)
	return err
}

// GetAllOrders 获取所有支付订单列表（管理员，带分页和筛选，含用户手机号）
func (r *PaymentOrderRepository) GetAllOrders(page, pageSize int, status *int, userID *int64, channel *string, payOrderNo *string) ([]*model.PaymentOrder, int64, error) {
	offset := (page - 1) * pageSize

	// 构建查询条件（po. 前缀，避免与 user 联表后的列名歧义）
	whereClause := ""
	args := []interface{}{}

	if status != nil {
		whereClause = "WHERE po.status = ?"
		args = append(args, *status)
	}

	if userID != nil {
		if whereClause == "" {
			whereClause = "WHERE po.user_id = ?"
		} else {
			whereClause += " AND po.user_id = ?"
		}
		args = append(args, *userID)
	}

	if channel != nil && *channel != "" {
		if whereClause == "" {
			whereClause = "WHERE po.channel = ?"
		} else {
			whereClause += " AND po.channel = ?"
		}
		args = append(args, *channel)
	}

	if payOrderNo != nil && *payOrderNo != "" {
		if whereClause == "" {
			whereClause = "WHERE po.pay_order_no = ?"
		} else {
			whereClause += " AND po.pay_order_no = ?"
		}
		args = append(args, *payOrderNo)
	}

	// 查询总数
	countQuery := "SELECT COUNT(*) FROM " + model.SysDB + ".payment_order po JOIN " + model.SysDB + ".user u ON u.id = po.user_id " + whereClause
	var total int64
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 查询列表
	query := `SELECT po.id, po.pay_order_no, po.user_id, po.amount, 
		po.channel, COALESCE(po.channel_trade_no, ''), COALESCE(po.bank_serial_no, ''), po.status, po.expire_time, po.paid_at, po.created_at, po.updated_at, 
		COALESCE(po.refund_status, 0), COALESCE(po.refund_amount, 0), po.refunded_at, po.intent, COALESCE(po.biz_no, ''), 
		COALESCE(po.balance_amount, 0), COALESCE(po.stock_reserved, 0), COALESCE(po.pay_info, ''), u.phone 
		FROM ` + model.SysDB + `.payment_order po 
		JOIN ` + model.SysDB + `.user u ON u.id = po.user_id
		` + whereClause + ` ORDER BY po.created_at DESC LIMIT ? OFFSET ?`

	args = append(args, pageSize, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	orders := make([]*model.PaymentOrder, 0)
	for rows.Next() {
		order := &model.PaymentOrder{}
		err := rows.Scan(
			&order.ID,
			&order.PayOrderNo,
			&order.UserID,
			&order.Amount,
			&order.Channel,
			&order.ChannelTradeNo,
			&order.BankSerialNo,
			&order.Status,
			&order.ExpireTime,
			&order.PaidAt,
			&order.CreatedAt,
			&order.UpdatedAt,
			&order.RefundStatus,
			&order.RefundAmount,
			&order.RefundedAt,
			&order.Intent,
			&order.BizNo,
			&order.BalanceAmount,
			&order.StockReserved,
			&order.PayInfo,
			&order.UserPhone,
		)
		if err != nil {
			return nil, 0, err
		}
		orders = append(orders, order)
	}

	return orders, total, nil
}
