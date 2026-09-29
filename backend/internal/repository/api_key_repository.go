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
