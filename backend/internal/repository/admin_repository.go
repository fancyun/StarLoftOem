package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
)

type AdminRepository struct {
	db *sql.DB
}

func NewAdminRepository(db *sql.DB) *AdminRepository {
	return &AdminRepository{db: db}
}

// GetAdminByUsername 根据用户名查询管理员
func (r *AdminRepository) GetAdminByUsername(username string) (*model.AdminUser, error) {
	query := `SELECT id, username, password_hash, nickname, status, permissions, last_login_at, created_at, updated_at 
		FROM ` + model.SysDB + `.admin_user WHERE username = ?`

	admin := &model.AdminUser{}
	err := r.db.QueryRow(query, username).Scan(
		&admin.ID,
		&admin.Username,
		&admin.PasswordHash,
		&admin.Nickname,
		&admin.Status,
		&admin.Permissions,
		&admin.LastLoginAt,
		&admin.CreatedAt,
		&admin.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return admin, nil
}

// UpdateLastLoginTime 更新最后登录时间
func (r *AdminRepository) UpdateLastLoginTime(adminID int64) error {
	query := `UPDATE ` + model.SysDB + `.admin_user SET last_login_at = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), time.Now(), adminID)
	return err
}

// GetAdminByID 根据ID查询管理员
func (r *AdminRepository) GetAdminByID(id int64) (*model.AdminUser, error) {
	query := `SELECT id, username, password_hash, nickname, status, aff_code, permissions, last_login_at, created_at, updated_at 
		FROM ` + model.SysDB + `.admin_user WHERE id = ?`

	admin := &model.AdminUser{}
	err := r.db.QueryRow(query, id).Scan(
		&admin.ID,
		&admin.Username,
		&admin.PasswordHash,
		&admin.Nickname,
		&admin.Status,
		&admin.AffCode,
		&admin.Permissions,
		&admin.LastLoginAt,
		&admin.CreatedAt,
		&admin.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return admin, nil
}

// UpdateAdminPassword 更新管理员密码
func (r *AdminRepository) UpdateAdminPassword(id int64, passwordHash string) error {
	query := `UPDATE ` + model.SysDB + `.admin_user SET password_hash = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, passwordHash, time.Now(), id)
	return err
}

// adminColumns 管理员查询列（统一维护，避免各处 SELECT 漏列 permissions）
const adminColumns = `id, username, password_hash, nickname, status, permissions, last_login_at, created_at, updated_at`

// ListAdmins 分页查询后台账号（员工管理用）
func (r *AdminRepository) ListAdmins(page, pageSize int) ([]*model.AdminUser, int64, error) {
	var total int64
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM ` + model.SysDB + `.admin_user`).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(`SELECT `+adminColumns+` FROM `+model.SysDB+`.admin_user ORDER BY id ASC LIMIT ? OFFSET ?`,
		pageSize, (page-1)*pageSize)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*model.AdminUser, 0)
	for rows.Next() {
		a := &model.AdminUser{}
		if err := rows.Scan(&a.ID, &a.Username, &a.PasswordHash, &a.Nickname, &a.Status, &a.Permissions,
			&a.LastLoginAt, &a.CreatedAt, &a.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, a)
	}
	return list, total, rows.Err()
}

// CreateAdmin 新建后台账号（成功回填自增 ID）
func (r *AdminRepository) CreateAdmin(a *model.AdminUser) error {
	result, err := r.db.Exec(`INSERT INTO `+model.SysDB+`.admin_user (username, password_hash, nickname, status, permissions, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		a.Username, a.PasswordHash, a.Nickname, a.Status, a.Permissions, time.Now(), time.Now())
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}

// UpdateAdminNickname 更新账号昵称
func (r *AdminRepository) UpdateAdminNickname(id int64, nickname string) error {
	_, err := r.db.Exec(`UPDATE `+model.SysDB+`.admin_user SET nickname = ?, updated_at = ? WHERE id = ?`, nickname, time.Now(), id)
	return err
}

// UpdateAdminStatus 启用/停用账号（1-启用 0-停用）
func (r *AdminRepository) UpdateAdminStatus(id int64, status int) error {
	_, err := r.db.Exec(`UPDATE `+model.SysDB+`.admin_user SET status = ?, updated_at = ? WHERE id = ?`, status, time.Now(), id)
	return err
}

// UpdateAdminPermissions 更新账号权限清单（逗号分隔权限码，all=全部）
func (r *AdminRepository) UpdateAdminPermissions(id int64, permissions string) error {
	_, err := r.db.Exec(`UPDATE `+model.SysDB+`.admin_user SET permissions = ?, updated_at = ? WHERE id = ?`, permissions, time.Now(), id)
	return err
}

// GetAdminByAffCode 按推广码查询员工账号（注册 ?ref= 绑定归属用；未命中返回 ErrUserNotFound）
func (r *AdminRepository) GetAdminByAffCode(code string) (*model.AdminUser, error) {
	if code == "" {
		return nil, ErrUserNotFound
	}
	a := &model.AdminUser{}
	err := r.db.QueryRow(`SELECT id, username, nickname, status FROM `+model.SysDB+`.admin_user WHERE aff_code = ? LIMIT 1`, code).Scan(
		&a.ID, &a.Username, &a.Nickname, &a.Status,
	)
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return a, nil
}

// SetAdminAffCode 写入员工推广码（开通销售身份时生成）
func (r *AdminRepository) SetAdminAffCode(adminID int64, code string) error {
	_, err := r.db.Exec(`UPDATE `+model.SysDB+`.admin_user SET aff_code = ?, updated_at = ? WHERE id = ?`, code, time.Now(), adminID)
	return err
}

// AffCodeExists 判断推广码是否已被员工账号占用（生成时查重）
func (r *AdminRepository) AffCodeExists(code string) (bool, error) {
	var n int
	if err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.admin_user WHERE aff_code = ?`, code).Scan(&n); err != nil {
		return false, err
	}
	return n > 0, nil
}
