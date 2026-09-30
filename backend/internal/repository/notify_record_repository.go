package repository

import (
	"database/sql"
	"strconv"
	"time"

	"oemrpa/internal/model"
)

// notifyRecordColumns 通知重试记录查询列（不含自增主键：该表以 biz_type + biz_no 为复合主键）
const notifyRecordColumns = `biz_type, biz_no, record_id, user_id, target_url, payload, status, fail_times, last_error, next_retry_at, created_at, updated_at`

type NotifyRecordRepository struct {
	db *sql.DB
}

func NewNotifyRecordRepository(db *sql.DB) *NotifyRecordRepository {
	return &NotifyRecordRepository{db: db}
}

// Upsert 落库待重推通知：以 (biz_type, biz_no) 为键幂等写入——同一业务单号重复失败只保留一行并重置重试进度
func (r *NotifyRecordRepository) Upsert(rec *model.NotifyRecord) error {
	_, err := r.db.Exec(
		`INSERT INTO `+model.SysDB+`.notify_record
			(biz_type, biz_no, record_id, user_id, target_url, payload, status, fail_times, last_error, next_retry_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, 0, '', NULL, ?, ?)
			ON DUPLICATE KEY UPDATE
				record_id = VALUES(record_id), user_id = VALUES(user_id),
				target_url = VALUES(target_url), payload = VALUES(payload),
				status = VALUES(status), fail_times = 0, last_error = '', next_retry_at = NULL,
				updated_at = VALUES(updated_at)`,
		rec.BizType, rec.BizNo, rec.RecordID, rec.UserID, rec.TargetURL, rec.Payload,
		model.NotifyPending, time.Now(), time.Now(),
	)
	return err
}

// ListRetryable 查询到期待推记录（status=0 且 next_retry_at 为空或已到期），最多 limit 条
func (r *NotifyRecordRepository) ListRetryable(now time.Time, limit int) ([]*model.NotifyRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(
		`SELECT `+notifyRecordColumns+`
			FROM `+model.SysDB+`.notify_record
			WHERE status = 0 AND (next_retry_at IS NULL OR next_retry_at <= ?)
			ORDER BY created_at ASC LIMIT ?`,
		now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanNotifyRecords(rows)
}

// UpdateResult 更新重推结果：失败累计次数并按指数退避写入下次时间（超限置 2 放弃）
func (r *NotifyRecordRepository) UpdateResult(bizType, bizNo string, status, failTimes int, lastErr string, nextRetryAt *time.Time) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SysDB+`.notify_record SET status = ?, fail_times = ?, last_error = ?, next_retry_at = ?, updated_at = ?
			WHERE biz_type = ? AND biz_no = ?`,
		status, failTimes, lastErr, nextRetryAt, time.Now(), bizType, bizNo,
	)
	return err
}

// Delete 删除通知记录（推送成功后不再保留成功历史）
func (r *NotifyRecordRepository) Delete(bizType, bizNo string) error {
	_, err := r.db.Exec(
		`DELETE FROM `+model.SysDB+`.notify_record WHERE biz_type = ? AND biz_no = ?`, bizType, bizNo,
	)
	return err
}

// Get 按业务类型 + 业务单号查询单条（未命中返回 (nil, nil)）
func (r *NotifyRecordRepository) Get(bizType, bizNo string) (*model.NotifyRecord, error) {
	rec := &model.NotifyRecord{}
	err := r.db.QueryRow(
		`SELECT `+notifyRecordColumns+` FROM `+model.SysDB+`.notify_record WHERE biz_type = ? AND biz_no = ?`,
		bizType, bizNo,
	).Scan(&rec.BizType, &rec.BizNo, &rec.RecordID, &rec.UserID, &rec.TargetURL, &rec.Payload, &rec.Status,
		&rec.FailTimes, &rec.LastError, &rec.NextRetryAt, &rec.CreatedAt, &rec.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return rec, nil
}

// ListRecords 后台分页查询通知记录（status/bizType 为空表示不限；keyword 为数字时按关联记录 ID 精确匹配，
// 否则按业务单号/目标地址模糊匹配）
func (r *NotifyRecordRepository) ListRecords(status *int, bizType, keyword, startDate, endDate string, page, pageSize int) ([]*model.NotifyRecord, int64, error) {
	where := "1=1"
	args := []interface{}{}
	if status != nil {
		where += " AND status = ?"
		args = append(args, *status)
	}
	if bizType != "" {
		where += " AND biz_type = ?"
		args = append(args, bizType)
	}
	if keyword != "" {
		if id, err := strconv.ParseInt(keyword, 10, 64); err == nil {
			where += " AND record_id = ?"
			args = append(args, id)
		} else {
			where += " AND (biz_no LIKE ? OR target_url LIKE ?)"
			args = append(args, "%"+keyword+"%", "%"+keyword+"%")
		}
	}
	if startDate != "" {
		where += " AND created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	if err := r.db.QueryRow("SELECT COUNT(*) FROM "+model.SysDB+".notify_record WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT ` + notifyRecordColumns + ` FROM ` + model.SysDB + `.notify_record WHERE ` + where +
		` ORDER BY created_at DESC, biz_no DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	list, err := scanNotifyRecords(rows)
	if err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

// ResetForRetry 把已放弃的记录重置为待推（失败次数与错误清空、立即纳入重试窗口）
func (r *NotifyRecordRepository) ResetForRetry(bizType, bizNo string) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SysDB+`.notify_record SET status = ?, fail_times = 0, last_error = '', next_retry_at = NULL, updated_at = ?
			WHERE biz_type = ? AND biz_no = ?`,
		model.NotifyPending, time.Now(), bizType, bizNo,
	)
	return err
}

// CountByStatus 按状态统计通知记录数（后台监控页展示待推/成功/已放弃）
func (r *NotifyRecordRepository) CountByStatus() (map[int]int64, error) {
	rows, err := r.db.Query(`SELECT status, COUNT(*) FROM ` + model.SysDB + `.notify_record GROUP BY status`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[int]int64)
	for rows.Next() {
		var status int
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		out[status] = count
	}
	return out, rows.Err()
}

// scanNotifyRecords 扫描通知记录行集
func scanNotifyRecords(rows *sql.Rows) ([]*model.NotifyRecord, error) {
	list := make([]*model.NotifyRecord, 0)
	for rows.Next() {
		rec := &model.NotifyRecord{}
		if err := rows.Scan(&rec.BizType, &rec.BizNo, &rec.RecordID, &rec.UserID, &rec.TargetURL, &rec.Payload,
			&rec.Status, &rec.FailTimes, &rec.LastError, &rec.NextRetryAt, &rec.CreatedAt, &rec.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, rec)
	}
	return list, rows.Err()
}
