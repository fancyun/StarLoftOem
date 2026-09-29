package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
)

type NotifyRecordRepository struct {
	db *sql.DB
}

func NewNotifyRecordRepository(db *sql.DB) *NotifyRecordRepository {
	return &NotifyRecordRepository{db: db}
}

// Create 落库待重推通知记录
func (r *NotifyRecordRepository) Create(rec *model.NotifyRecord) error {
	_, err := r.db.Exec(
		`INSERT INTO `+model.SysDB+`.notify_record
			(biz_type, record_id, user_id, target_url, payload, status, fail_times, last_error, next_retry_at, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.BizType, rec.RecordID, rec.UserID, rec.TargetURL, rec.Payload, rec.Status, rec.FailTimes,
		rec.LastError, rec.NextRetryAt, time.Now(), time.Now(),
	)
	return err
}

// ListRetryable 查询到期待推记录（status=0 且 next_retry_at 为空或已到期），最多 limit 条
func (r *NotifyRecordRepository) ListRetryable(now time.Time, limit int) ([]*model.NotifyRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.db.Query(
		`SELECT id, biz_type, record_id, user_id, target_url, payload, status, fail_times, last_error, next_retry_at, created_at, updated_at
			FROM `+model.SysDB+`.notify_record
			WHERE status = 0 AND (next_retry_at IS NULL OR next_retry_at <= ?)
			ORDER BY id ASC LIMIT ?`,
		now, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*model.NotifyRecord, 0)
	for rows.Next() {
		rec := &model.NotifyRecord{}
		if err := rows.Scan(
			&rec.ID, &rec.BizType, &rec.RecordID, &rec.UserID, &rec.TargetURL, &rec.Payload, &rec.Status,
			&rec.FailTimes, &rec.LastError, &rec.NextRetryAt, &rec.CreatedAt, &rec.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, rec)
	}
	return list, nil
}

// UpdateResult 更新重推结果：成功置 1；失败累计次数并按指数退避计算下次时间（超限置 2 放弃）
func (r *NotifyRecordRepository) UpdateResult(id int64, status int, failTimes int, lastErr string, nextRetryAt *time.Time) error {
	_, err := r.db.Exec(
		`UPDATE `+model.SysDB+`.notify_record SET status = ?, fail_times = ?, last_error = ?, next_retry_at = ?, updated_at = ? WHERE id = ?`,
		status, failTimes, lastErr, nextRetryAt, time.Now(), id,
	)
	return err
}
