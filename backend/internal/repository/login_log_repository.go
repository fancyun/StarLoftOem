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

// ListUserLoginLogs 后台分页查询用户登录日志（keyword 模糊匹配账号/IP；status 为空表示不限）
func (r *LoginLogRepository) ListUserLoginLogs(keyword string, status *int, startDate, endDate string, page, pageSize int) ([]*model.UserLoginLog, int64, error) {
	where, args := loginLogWhere("(account LIKE ? OR ip LIKE ?)", keyword, status, startDate, endDate)
	var total int64
	if err := r.db.QueryRow("SELECT COUNT(*) FROM "+model.SysDB+".user_login_log WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT id, user_id, account, login_type, ip, user_agent, status, fail_reason, created_at
		FROM ` + model.SysDB + `.user_login_log WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.UserLoginLog, 0, pageSize)
	for rows.Next() {
		var l model.UserLoginLog
		if err := rows.Scan(&l.ID, &l.UserID, &l.Account, &l.LoginType, &l.IP, &l.UserAgent,
			&l.Status, &l.FailReason, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &l)
	}
	return list, total, rows.Err()
}

// ListAdminLoginLogs 后台分页查询管理员登录日志（keyword 模糊匹配账号/IP；status 为空表示不限）
func (r *LoginLogRepository) ListAdminLoginLogs(keyword string, status *int, startDate, endDate string, page, pageSize int) ([]*model.AdminLoginLog, int64, error) {
	where, args := loginLogWhere("(username LIKE ? OR ip LIKE ?)", keyword, status, startDate, endDate)
	var total int64
	if err := r.db.QueryRow("SELECT COUNT(*) FROM "+model.SysDB+".admin_login_log WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	query := `SELECT id, admin_id, username, ip, user_agent, status, fail_reason, created_at
		FROM ` + model.SysDB + `.admin_login_log WHERE ` + where + ` ORDER BY id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.AdminLoginLog, 0, pageSize)
	for rows.Next() {
		var l model.AdminLoginLog
		if err := rows.Scan(&l.ID, &l.AdminID, &l.Username, &l.IP, &l.UserAgent,
			&l.Status, &l.FailReason, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, &l)
	}
	return list, total, rows.Err()
}

// loginLogWhere 组装登录日志查询条件（keyword 为空不拼接 LIKE，故 keywordField 由调用方按表字段给定）
func loginLogWhere(keywordField, keyword string, status *int, startDate, endDate string) (string, []interface{}) {
	where := "1=1"
	args := []interface{}{}
	if keyword != "" {
		like := "%" + keyword + "%"
		where += " AND " + keywordField
		args = append(args, like, like)
	}
	if status != nil {
		where += " AND status = ?"
		args = append(args, *status)
	}
	if startDate != "" {
		where += " AND created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}
	return where, args
}
