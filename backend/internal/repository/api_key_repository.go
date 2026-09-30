package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
	"oemrpa/internal/utils"
)

// ApiKeyRepository 平台 API 密钥存取（位于 sys 库 api 表，一个账号可持有多把密钥）
type ApiKeyRepository struct {
	db *sql.DB
}

func NewApiKeyRepository(db *sql.DB) *ApiKeyRepository {
	return &ApiKeyRepository{db: db}
}

const apiKeyColumns = `id, user_id, name, api_key, api_secret, permission, created_at, updated_at`

func scanApiKey(row interface{ Scan(...interface{}) error }) (*model.ApiKey, error) {
	k := &model.ApiKey{}
	err := row.Scan(&k.ID, &k.UserID, &k.Name, &k.APIKey, &k.APISecret, &k.Permission, &k.CreatedAt, &k.UpdatedAt)
	if err != nil {
		return nil, err
	}
	// API Secret 存储加密，读取时解密供 HMAC 验签/展示
	k.APISecret = utils.DecryptText(k.APISecret)
	return k, nil
}

// GetByAPIKey 根据 API Key 查询密钥记录
func (r *ApiKeyRepository) GetByAPIKey(apiKey string) (*model.ApiKey, error) {
	query := `SELECT ` + apiKeyColumns + ` FROM ` + model.SysDB + `.api WHERE api_key = ?`
	k, err := scanApiKey(r.db.QueryRow(query, apiKey))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

// GetByID 按密钥 ID 查询（下游回调按发起该请求的密钥签名）
func (r *ApiKeyRepository) GetByID(id int64) (*model.ApiKey, error) {
	query := `SELECT ` + apiKeyColumns + ` FROM ` + model.SysDB + `.api WHERE id = ?`
	k, err := scanApiKey(r.db.QueryRow(query, id))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

// GetByUser 查询用户最早创建的密钥（账号主密钥：历史记录未记录发起密钥时的回调签名回退）
func (r *ApiKeyRepository) GetByUser(userID int64) (*model.ApiKey, error) {
	query := `SELECT ` + apiKeyColumns + ` FROM ` + model.SysDB + `.api WHERE user_id = ? ORDER BY id ASC LIMIT 1`
	k, err := scanApiKey(r.db.QueryRow(query, userID))
	if err == sql.ErrNoRows {
		return nil, ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return k, nil
}

// ListByUser 查询用户全部密钥（按创建顺序）
func (r *ApiKeyRepository) ListByUser(userID int64) ([]*model.ApiKey, error) {
	query := `SELECT ` + apiKeyColumns + ` FROM ` + model.SysDB + `.api WHERE user_id = ? ORDER BY id ASC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*model.ApiKey, 0)
	for rows.Next() {
		k, err := scanApiKey(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, k)
	}
	return list, rows.Err()
}

// CountByUser 统计用户已有密钥数量（创建时校验上限）
func (r *ApiKeyRepository) CountByUser(userID int64) (int, error) {
	var n int
	err := r.db.QueryRow(`SELECT COUNT(*) FROM `+model.SysDB+`.api WHERE user_id = ?`, userID).Scan(&n)
	return n, err
}

// Create 新增一条 API 密钥（Secret 存储加密）
func (r *ApiKeyRepository) Create(k *model.ApiKey) error {
	query := `INSERT INTO ` + model.SysDB + `.api (user_id, name, api_key, api_secret, permission, created_at, updated_at) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`
	res, err := r.db.Exec(query, k.UserID, k.Name, k.APIKey, utils.EncryptText(k.APISecret), k.Permission, time.Now(), time.Now())
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err == nil {
		k.ID = id
	}
	return nil
}

// UpdateKey 更新指定密钥的名称与权限范围（限本人密钥）
func (r *ApiKeyRepository) UpdateKey(id, userID int64, name, permission string) error {
	_, err := r.db.Exec(`UPDATE `+model.SysDB+`.api SET name = ?, permission = ?, updated_at = ? WHERE id = ? AND user_id = ?`,
		name, permission, time.Now(), id, userID)
	return err
}

// DeleteByID 删除指定密钥（限本人密钥）
func (r *ApiKeyRepository) DeleteByID(id, userID int64) error {
	_, err := r.db.Exec(`DELETE FROM `+model.SysDB+`.api WHERE id = ? AND user_id = ?`, id, userID)
	return err
}

// ApiKeyRow 后台密钥列表行（绝不包含 api_secret，含归属用户信息）
type ApiKeyRow struct {
	ID         int64     `json:"id"`
	UserID     int64     `json:"user_id"`
	Phone      string    `json:"phone"`
	Username   string    `json:"username"`
	Name       string    `json:"name"`
	APIKey     string    `json:"api_key"`
	Permission string    `json:"permission"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ListPage 后台分页查询全平台 API 密钥（只读）。
// 查询列显式列出且不含 api_secret：从查询层断掉密钥明文外泄路径。
func (r *ApiKeyRepository) ListPage(userID *int64, keyword, startDate, endDate string, page, pageSize int) ([]*ApiKeyRow, int64, error) {
	where := "1=1"
	args := []interface{}{}
	if userID != nil {
		where += " AND k.user_id = ?"
		args = append(args, *userID)
	}
	if keyword != "" {
		like := "%" + keyword + "%"
		where += " AND (k.api_key LIKE ? OR k.name LIKE ?)"
		args = append(args, like, like)
	}
	if startDate != "" {
		where += " AND k.created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND k.created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	countSQL := `SELECT COUNT(*) FROM ` + model.SysDB + `.api k LEFT JOIN ` + model.SysDB + `.user u ON u.id = k.user_id WHERE ` + where
	if err := r.db.QueryRow(countSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT k.id, k.user_id, COALESCE(u.phone,''), COALESCE(u.username,''),
			k.name, k.api_key, k.permission, k.created_at, k.updated_at
		FROM ` + model.SysDB + `.api k
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = k.user_id
		WHERE ` + where + ` ORDER BY k.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*ApiKeyRow, 0, pageSize)
	for rows.Next() {
		row := &ApiKeyRow{}
		if err := rows.Scan(&row.ID, &row.UserID, &row.Phone, &row.Username, &row.Name, &row.APIKey,
			&row.Permission, &row.CreatedAt, &row.UpdatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, row)
	}
	return list, total, rows.Err()
}
