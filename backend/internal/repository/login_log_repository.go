package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
)

type LoginLogRepository struct {
	db *sql.DB
}

func NewLoginLogRepository(db *sql.DB) *LoginLogRepository {
	return &LoginLogRepository{db: db}
}

// InsertUserLoginLog 记录用户登录日志（系统库）
func (r *LoginLogRepository) InsertUserLoginLog(log *model.UserLoginLog) error {
	query := `INSERT INTO ` + model.SysDB + `.user_login_log
		(user_id, account, login_type, ip, user_agent, status, fail_reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query,
		log.UserID, log.Account, log.LoginType, log.IP, log.UserAgent,
		log.Status, log.FailReason, time.Now(),
	)
	return err
}

// InsertAdminLoginLog 记录管理员登录日志（系统库）
func (r *LoginLogRepository) InsertAdminLoginLog(log *model.AdminLoginLog) error {
	query := `INSERT INTO ` + model.SysDB + `.admin_login_log
		(admin_id, username, ip, user_agent, status, fail_reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	_, err := r.db.Exec(query,
		log.AdminID, log.Username, log.IP, log.UserAgent,
		log.Status, log.FailReason, time.Now(),
	)
	return err
}
