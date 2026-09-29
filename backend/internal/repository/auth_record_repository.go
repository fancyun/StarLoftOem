package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
)

type AuthRecordRepository struct {
	db     *sql.DB
	dbName string // 认证记录所属库名（oem_fv）
}

func NewAuthRecordRepository(db *sql.DB, dbName string) *AuthRecordRepository {
	return &AuthRecordRepository{db: db, dbName: dbName}
}

// CreateRecord 创建认证记录
func (r *AuthRecordRepository) CreateRecord(record *model.AuthRecord) error {
	query := `INSERT INTO ` + r.dbName + `.auth_record
		(biz_no, user_id, name, id_card, return_url, notify_url, 
		biz_extra_data, status, cost, pay_type, user_pack_id, pack_count, product, is_refunded, notify_times, 
			notify_status, created_at, updated_at) 
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	result, err := r.db.Exec(query,
		record.BizNo,
		record.UserID,
		record.Name,
		record.IDCard,
		record.ReturnURL,
		record.NotifyURL,
		record.BizExtraData,
		record.Status,
		record.Cost,
		record.PayType,
		record.UserPackID,
		record.PackCount,
		record.Product,
		record.IsRefunded,
		record.NotifyTimes,
		record.NotifyStatus,
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
	record.ID = id
	return nil
}

// recordColumns 认证记录通用查询列
const recordColumns = `id, biz_no, user_id, 
	COALESCE(name, ''), COALESCE(id_card, ''),
	COALESCE(return_url, ''), COALESCE(notify_url, ''), COALESCE(biz_extra_data, ''), 
	COALESCE(up_token, ''), COALESCE(up_biz_id, ''), COALESCE(up_request_id, ''), token_expire_at, COALESCE(up_query_count, 0), 
	COALESCE(result_code, ''), COALESCE(result_message, ''),
	status, cost, pay_type, COALESCE(user_pack_id, 0), COALESCE(pack_count, 0), COALESCE(product, ''),
	is_refunded, notify_times, 
	notify_status, COALESCE(best_img_fetched, 0), COALESCE(media_dir, ''),
	created_at, updated_at, finished_at, media_expire_at`

// scanRecord 将查询结果扫描到 AuthRecord
func scanRecord(row interface{ Scan(...interface{}) error }) (*model.AuthRecord, error) {
	record := &model.AuthRecord{}
	err := row.Scan(
		&record.ID,
		&record.BizNo,
		&record.UserID,
		&record.Name,
		&record.IDCard,
		&record.ReturnURL,
		&record.NotifyURL,
		&record.BizExtraData,
		&record.UpToken,
		&record.UpBizID,
		&record.UpRequestID,
		&record.TokenExpireAt,
		&record.UpQueryCount,
		&record.ResultCode,
		&record.ResultMessage,
		&record.Status,
		&record.Cost,
		&record.PayType,
		&record.UserPackID,
		&record.PackCount,
		&record.Product,
		&record.IsRefunded,
		&record.NotifyTimes,
		&record.NotifyStatus,
		&record.BestImgFetched,
		&record.MediaDir,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.FinishedAt,
		&record.MediaExpireAt,
	)
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetRecordByBizNo 根据唯一业务流水号查询订单
func (r *AuthRecordRepository) GetRecordByBizNo(bizNo string) (*model.AuthRecord, error) {
	query := `SELECT ` + recordColumns + ` 
		FROM ` + r.dbName + `.auth_record WHERE biz_no = ?`

	record, err := scanRecord(r.db.QueryRow(query, bizNo))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetRecordByID 根据订单ID查询订单
func (r *AuthRecordRepository) GetRecordByID(recordID int64) (*model.AuthRecord, error) {
	query := `SELECT ` + recordColumns + `, COALESCE(result_data, '')
		FROM ` + r.dbName + `.auth_record WHERE id = ?`

	record := &model.AuthRecord{}
	err := r.db.QueryRow(query, recordID).Scan(
		&record.ID,
		&record.BizNo,
		&record.UserID,
		&record.Name,
		&record.IDCard,
		&record.ReturnURL,
		&record.NotifyURL,
		&record.BizExtraData,
		&record.UpToken,
		&record.UpBizID,
		&record.UpRequestID,
		&record.TokenExpireAt,
		&record.UpQueryCount,
		&record.ResultCode,
		&record.ResultMessage,
		&record.Status,
		&record.Cost,
		&record.PayType,
		&record.UserPackID,
		&record.PackCount,
		&record.Product,
		&record.IsRefunded,
		&record.NotifyTimes,
		&record.NotifyStatus,
		&record.BestImgFetched,
		&record.MediaDir,
		&record.CreatedAt,
		&record.UpdatedAt,
		&record.FinishedAt,
		&record.MediaExpireAt,
		&record.ResultData,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// RecentAuthRecord 最近认证记录（含用户手机号和姓名）
type RecentAuthRecord struct {
	BizNo     string
	UserPhone string
	Name      string
	Status    int
	Cost      float64
	CreatedAt time.Time
}

// GetRecentRecords 获取最近认证记录列表（含用户手机号和姓名）
func (r *AuthRecordRepository) GetRecentRecords(limit int) ([]*RecentAuthRecord, error) {
	query := `SELECT ao.biz_no, u.phone, 
		COALESCE((SELECT kr.name FROM ` + model.SysDB + `.kyc kr WHERE kr.user_id = ao.user_id ORDER BY kr.id DESC LIMIT 1), ''), 
		ao.status, ao.cost, ao.created_at
		FROM ` + r.dbName + `.auth_record ao
		JOIN ` + model.SysDB + `.user u ON u.id = ao.user_id
		ORDER BY ao.created_at DESC LIMIT ?`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*RecentAuthRecord, 0)
	for rows.Next() {
		o := &RecentAuthRecord{}
		err := rows.Scan(&o.BizNo, &o.UserPhone, &o.Name, &o.Status, &o.Cost, &o.CreatedAt)
		if err != nil {
			return nil, err
		}
		records = append(records, o)
	}
	return records, nil
}

// GetDailyRecordStats 按天统计认证记录数
func (r *AuthRecordRepository) GetDailyRecordStats(startDate, endDate string) (map[string]int64, error) {
	query := `SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS d, COUNT(*) AS c
		FROM ` + r.dbName + `.auth_record
		WHERE DATE(created_at) >= ? AND DATE(created_at) <= ?
		GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d')`

	rows, err := r.db.Query(query, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var d string
		var c int64
		if err := rows.Scan(&d, &c); err != nil {
			return nil, err
		}
		result[d] = c
	}
	return result, nil
}

// GetDailyIncomeStats 按天统计认证收入（仅统计已完成的认证记录）
func (r *AuthRecordRepository) GetDailyIncomeStats(startDate, endDate string) (map[string]float64, error) {
	query := `SELECT DATE_FORMAT(created_at, '%Y-%m-%d') AS d, COALESCE(SUM(cost), 0) AS amount
		FROM ` + r.dbName + `.auth_record
		WHERE status = 2 AND DATE(created_at) >= ? AND DATE(created_at) <= ?
		GROUP BY DATE_FORMAT(created_at, '%Y-%m-%d')`

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

// UpdateRecordUpstreamInfo 更新订单上游信息
// expireAt 为上游 token 到期时间（可为 nil，表示上游未返回）
func (r *AuthRecordRepository) UpdateRecordUpstreamInfo(recordID int64, token, bizID, requestID string, expireAt *time.Time) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET up_token = ?, up_biz_id = ?, up_request_id = ?, token_expire_at = ?, status = 1, updated_at = ? 
		WHERE id = ?`
	_, err := r.db.Exec(query, token, bizID, requestID, expireAt, time.Now(), recordID)
	return err
}

// GetRecordByUpBizID 根据上游业务ID查询订单
func (r *AuthRecordRepository) GetRecordByUpBizID(upBizID string) (*model.AuthRecord, error) {
	query := `SELECT ` + recordColumns + ` 
		FROM ` + r.dbName + `.auth_record WHERE up_biz_id = ?`

	record, err := scanRecord(r.db.QueryRow(query, upBizID))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// GetRecordByUpToken 根据上游 token 查询订单（人脸核验自站 return 回调用）
func (r *AuthRecordRepository) GetRecordByUpToken(upToken string) (*model.AuthRecord, error) {
	query := `SELECT ` + recordColumns + ` 
		FROM ` + r.dbName + `.auth_record WHERE up_token = ?`

	record, err := scanRecord(r.db.QueryRow(query, upToken))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return record, nil
}

// UpdateRecordCharge 回写订单计费结果：实际扣费金额（余额支付；资源包/未扣到为 0）、扣费方式、
// 命中的用户资源包 ID 与扣减次数。
func (r *AuthRecordRepository) UpdateRecordCharge(recordID int64, cost float64, payType int, userPackID int64, packCount int) error {
	query := `UPDATE ` + r.dbName + `.auth_record
		SET cost = ?, pay_type = ?, user_pack_id = ?, pack_count = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, cost, payType, userPackID, packCount, time.Now(), recordID)
	return err
}

// UpdateRecordResult 更新订单认证结果。
// 仅允许进行中订单（status 0/1）更新：终态订单（2 成功/3 失败/4 已取消/5 超时结束/6 发起失败）
// 一律拒绝改写，防止通过 return 轮询/回调重放反复刷新 finished_at
// （绕过活体最佳图 24h 领取窗口）、把成功订单降级为失败或触发重复退款。
func (r *AuthRecordRepository) UpdateRecordResult(recordID int64, resultCode, resultMessage string, status int) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET result_code = ?, result_message = ?, status = ?, finished_at = ?, updated_at = ? 
		WHERE id = ? AND status IN (0, 1)`
	_, err := r.db.Exec(query, resultCode, resultMessage, status, time.Now(), time.Now(), recordID)
	return err
}

// UpdateRecordResultForce 按上游结果改写认证记录（不限状态，终态记录也改写）。
// 仅供后台「查询结果」手动核对使用：由管理员主动发起，以最后一次上游结果为准修正本地记录。
func (r *AuthRecordRepository) UpdateRecordResultForce(recordID int64, resultCode, resultMessage string, status int) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET result_code = ?, result_message = ?, status = ?, updated_at = ? 
		WHERE id = ?`
	_, err := r.db.Exec(query, resultCode, resultMessage, status, time.Now(), recordID)
	return err
}

// UpdateRecordResultWithData 更新订单认证结果及完整结果数据（同步认证结果落库用）。
// 与 UpdateRecordResult 相同：仅允许进行中订单（status 0/1）更新，终态拒绝改写。
func (r *AuthRecordRepository) UpdateRecordResultWithData(recordID int64, resultCode, resultMessage string, status int, resultData string) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET result_code = ?, result_message = ?, status = ?, result_data = ?, finished_at = ?, updated_at = ? 
		WHERE id = ? AND status IN (0, 1)`
	_, err := r.db.Exec(query, resultCode, resultMessage, status, resultData, time.Now(), time.Now(), recordID)
	return err
}

// UpdateRecordRefundFlag 仅标记订单已退款（不改变状态）
func (r *AuthRecordRepository) UpdateRecordRefundFlag(recordID int64) error {
	query := `UPDATE ` + r.dbName + `.auth_record SET is_refunded = 1, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), recordID)
	return err
}

// IncrRecordUpQueryCount 递增订单「已向上游查询结果次数」并返回递增后的值。
// 上游 get_result 对同一 biz_id 仅允许 3 次调用（见 model.UpstreamQueryLimit），
// 调用前先计数做预算，超限即不再请求上游，避免第 4 次返回 DATA_DESTROYED 把结果数据打销毁。
func (r *AuthRecordRepository) IncrRecordUpQueryCount(recordID int64) (int, error) {
	query := `UPDATE ` + r.dbName + `.auth_record
		SET up_query_count = COALESCE(up_query_count, 0) + 1, updated_at = ? WHERE id = ?`
	if _, err := r.db.Exec(query, time.Now(), recordID); err != nil {
		return 0, err
	}

	var count int
	countQuery := `SELECT COALESCE(up_query_count, 0) FROM ` + r.dbName + `.auth_record WHERE id = ?`
	if err := r.db.QueryRow(countQuery, recordID).Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}

// GetPendingRecords 查询所有处理中且未退款的订单（供定时任务主动同步上游结果）
func (r *AuthRecordRepository) GetPendingRecords() ([]*model.AuthRecord, error) {
	query := `SELECT ` + recordColumns + ` 
		FROM ` + r.dbName + `.auth_record WHERE status IN (0, 1) AND is_refunded = 0 AND up_biz_id IS NOT NULL AND up_biz_id != ''`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*model.AuthRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

// GetExpiredPendingRecords 查询核身已超时但仍未终结的认证记录（供定时任务终结并退款）：
// 以 token_expire_at 为准，上游未返回到期时间时按 created_at + fallbackMinutes 分钟兜底。
func (r *AuthRecordRepository) GetExpiredPendingRecords(fallbackMinutes int) ([]*model.AuthRecord, error) {
	if fallbackMinutes <= 0 {
		fallbackMinutes = 15
	}
	query := `SELECT ` + recordColumns + ` 
		FROM ` + r.dbName + `.auth_record 
		WHERE status IN (0, 1) AND is_refunded = 0 
		  AND COALESCE(token_expire_at, DATE_ADD(created_at, INTERVAL ? MINUTE)) < ?`

	rows, err := r.db.Query(query, fallbackMinutes, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*model.AuthRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

// GetAllRecords 获取所有认证记录列表（管理员，带分页和筛选，含用户手机号）
func (r *AuthRecordRepository) GetAllRecords(page, pageSize int, status *int, userID *int64) ([]*model.AuthRecord, int64, error) {
	offset := (page - 1) * pageSize

	// 构建查询条件（带 ao. 前缀，避免与 user 联表后的列名歧义）
	whereClause := ""
	args := []interface{}{}

	if status != nil {
		whereClause = "WHERE ao.status = ?"
		args = append(args, *status)
	}

	if userID != nil {
		if whereClause == "" {
			whereClause = "WHERE ao.user_id = ?"
		} else {
			whereClause += " AND ao.user_id = ?"
		}
		args = append(args, *userID)
	}

	// 查询总数
	countQuery := "SELECT COUNT(*) FROM " + r.dbName + ".auth_record ao JOIN " + model.SysDB + ".user u ON u.id = ao.user_id " + whereClause
	var total int64
	err := r.db.QueryRow(countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	// 查询列表
	query := `SELECT ao.id, ao.biz_no, ao.user_id, 
		COALESCE(ao.name, ''), COALESCE(ao.id_card, ''),
		COALESCE(ao.return_url, ''), COALESCE(ao.notify_url, ''), COALESCE(ao.biz_extra_data, ''), 
		COALESCE(ao.up_token, ''), COALESCE(ao.up_biz_id, ''), COALESCE(ao.up_request_id, ''), ao.token_expire_at, COALESCE(ao.up_query_count, 0), 
		COALESCE(ao.result_code, ''), COALESCE(ao.result_message, ''),
		ao.status, ao.cost, ao.pay_type, COALESCE(ao.user_pack_id, 0), COALESCE(ao.pack_count, 0), COALESCE(ao.product, ''),
		ao.is_refunded, ao.notify_times, 
		ao.notify_status, COALESCE(ao.best_img_fetched, 0), COALESCE(ao.media_dir, ''),
		ao.created_at, ao.updated_at, ao.finished_at, ao.media_expire_at, u.phone 
		FROM ` + r.dbName + `.auth_record ao 
		JOIN ` + model.SysDB + `.user u ON u.id = ao.user_id
		` + whereClause + ` ORDER BY ao.created_at DESC LIMIT ? OFFSET ?`

	args = append(args, pageSize, offset)
	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	records := make([]*model.AuthRecord, 0)
	for rows.Next() {
		record := &model.AuthRecord{}
		err := rows.Scan(
			&record.ID,
			&record.BizNo,
			&record.UserID,
			&record.Name,
			&record.IDCard,
			&record.ReturnURL,
			&record.NotifyURL,
			&record.BizExtraData,
			&record.UpToken,
			&record.UpBizID,
			&record.UpRequestID,
			&record.TokenExpireAt,
			&record.UpQueryCount,
			&record.ResultCode,
			&record.ResultMessage,
			&record.Status,
			&record.Cost,
			&record.PayType,
			&record.UserPackID,
			&record.PackCount,
			&record.Product,
			&record.IsRefunded,
			&record.NotifyTimes,
			&record.NotifyStatus,
			&record.BestImgFetched,
			&record.MediaDir,
			&record.CreatedAt,
			&record.UpdatedAt,
			&record.FinishedAt,
			&record.MediaExpireAt,
			&record.UserPhone,
		)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, record)
	}

	return records, total, nil
}

// GetUserAuthRecords 获取指定用户的认证记录列表
func (r *AuthRecordRepository) GetUserAuthRecords(userID int64, page, pageSize int) ([]*model.AuthRecord, int64, error) {
	return r.GetUserAuthRecordsByDate(userID, page, pageSize, "", "")
}

// GetUserAuthRecordsByDate 获取指定用户的认证记录列表（可选按创建时间区间过滤，日期格式 YYYY-MM-DD）
func (r *AuthRecordRepository) GetUserAuthRecordsByDate(userID int64, page, pageSize int, startDate, endDate string) ([]*model.AuthRecord, int64, error) {
	where := `WHERE user_id = ?`
	args := []interface{}{userID}
	if startDate != "" {
		where += ` AND created_at >= ?`
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += ` AND created_at <= ?`
		args = append(args, endDate+" 23:59:59")
	}

	// 查询总数
	countQuery := `SELECT COUNT(*) FROM ` + r.dbName + `.auth_record ` + where
	var total int64
	if err := r.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 查询订单列表
	offset := (page - 1) * pageSize
	query := `SELECT ` + recordColumns + `
		FROM ` + r.dbName + `.auth_record ` + where + `
		ORDER BY created_at DESC 
		LIMIT ? OFFSET ?`

	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, offset)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	records := make([]*model.AuthRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, 0, err
		}
		records = append(records, record)
	}

	return records, total, nil
}

// UpdateBestImgFetched 标记认证记录的活体最佳图已领取（仅一次）
func (r *AuthRecordRepository) UpdateBestImgFetched(recordID int64) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET best_img_fetched = 1, updated_at = ? WHERE id = ? AND best_img_fetched = 0`
	res, err := r.db.Exec(query, time.Now(), recordID)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrUserNotFound // 已领取过（并发/重复请求）视为无效
	}
	return nil
}

// UpdateRecordIdentity 记录认证记录的姓名与身份证号
func (r *AuthRecordRepository) UpdateRecordIdentity(recordID int64, name, idCard string) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET name = ?, id_card = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, name, idCard, time.Now(), recordID)
	return err
}

// UpdateRecordMedia 记录认证记录已保存的媒体目录与过期时间
func (r *AuthRecordRepository) UpdateRecordMedia(recordID int64, mediaDir string, expireAt *time.Time) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET media_dir = ?, media_expire_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, mediaDir, expireAt, time.Now(), recordID)
	return err
}

// ClearRecordMedia 清空认证记录的媒体记录（媒体文件过期清理后）
func (r *AuthRecordRepository) ClearRecordMedia(recordID int64) error {
	query := `UPDATE ` + r.dbName + `.auth_record 
		SET media_dir = '', media_expire_at = NULL, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), recordID)
	return err
}

// GetExpiredMediaRecords 查询媒体已过期的认证记录
func (r *AuthRecordRepository) GetExpiredMediaRecords(now time.Time) ([]*model.AuthRecord, error) {
	query := `SELECT ` + recordColumns + ` 
		FROM ` + r.dbName + `.auth_record 
		WHERE media_dir != '' AND media_expire_at IS NOT NULL AND media_expire_at < ?`

	rows, err := r.db.Query(query, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	records := make([]*model.AuthRecord, 0)
	for rows.Next() {
		record, err := scanRecord(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}
