package repository

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"oemrpa/internal/model"
)

// 产品配置表所在库（仅允许这两个值，避免动态库名注入）
var allowedProductDBs = map[string]bool{model.FvDB: true, model.SmsDB: true}

var ErrSettingNotFound = errors.New("setting not found")

type SettingRepository struct {
	db *sql.DB
}

func NewSettingRepository(db *sql.DB) *SettingRepository {
	return &SettingRepository{db: db}
}

// ProductConfigItem 产品配置项（人脸核验库 / 短信库 product_config 通用）
type ProductConfigItem struct {
	ID          int64     `json:"id"`
	ConfigKey   string    `json:"config_key"`
	ConfigValue string    `json:"config_value"`
	Remark      string    `json:"remark"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AllSettings 读取系统库全部配置为 key-value 映射（供启动时覆盖代码内置默认值）
func (r *SettingRepository) AllSettings() (map[string]string, error) {
	rows, err := r.db.Query(`SELECT config_key, config_value FROM ` + model.SysDB + `.setting`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if k.Valid {
			out[k.String] = v.String
		}
	}
	return out, rows.Err()
}

// ListSettings 列出系统配置项（category 为空时返回全部）
func (r *SettingRepository) ListSettings(category string) ([]*model.Setting, error) {
	query := `SELECT id, config_key, config_value, category, remark, created_at, updated_at FROM ` + model.SysDB + `.setting`
	args := []interface{}{}
	if category != "" {
		query += ` WHERE category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY category, config_key`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*model.Setting
	for rows.Next() {
		it := &model.Setting{}
		var v sql.NullString
		if err := rows.Scan(&it.ID, &it.ConfigKey, &v, &it.Category, &it.Remark, &it.CreatedAt, &it.UpdatedAt); err != nil {
			return nil, err
		}
		it.ConfigValue = v.String
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpsertSetting 写入或更新系统配置项
func (r *SettingRepository) UpsertSetting(key, value, category, remark string) error {
	_, err := r.db.Exec(`INSERT INTO `+model.SysDB+`.setting (config_key, config_value, category, remark, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE config_value = VALUES(config_value), category = VALUES(category), remark = VALUES(remark), updated_at = NOW()`,
		key, value, category, remark)
	return err
}

// InsertSettingIfAbsent 仅在该键不存在时写入（`INSERT IGNORE`）：用于启动时预置配置目录，
// 保证不覆盖后台已修改的值，且后续版本新增的键能自动补齐。
func (r *SettingRepository) InsertSettingIfAbsent(key, value, category, remark string) error {
	_, err := r.db.Exec(`INSERT IGNORE INTO `+model.SysDB+`.setting (config_key, config_value, category, remark, created_at, updated_at)
		VALUES (?, ?, ?, ?, NOW(), NOW())`, key, value, category, remark)
	return err
}

// DeleteSetting 删除系统配置项（删除后该键回落代码内置默认值）
func (r *SettingRepository) DeleteSetting(key string) error {
	_, err := r.db.Exec(`DELETE FROM `+model.SysDB+`.setting WHERE config_key = ?`, key)
	return err
}

// ProductConfigs 读取指定产品库的全部配置为 key-value 映射
func (r *SettingRepository) ProductConfigs(dbName string) (map[string]string, error) {
	if !allowedProductDBs[dbName] {
		return nil, fmt.Errorf("不支持的产品库: %s", dbName)
	}
	rows, err := r.db.Query(`SELECT config_key, config_value FROM ` + dbName + `.product_config`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]string)
	for rows.Next() {
		var k, v sql.NullString
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		if k.Valid {
			out[k.String] = v.String
		}
	}
	return out, rows.Err()
}

// ListProductConfigs 列出指定产品库的配置项
func (r *SettingRepository) ListProductConfigs(dbName string) ([]*ProductConfigItem, error) {
	if !allowedProductDBs[dbName] {
		return nil, fmt.Errorf("不支持的产品库: %s", dbName)
	}
	query := `SELECT id, config_key, config_value, remark, updated_at FROM ` + dbName + `.product_config ORDER BY config_key`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*ProductConfigItem
	for rows.Next() {
		it := &ProductConfigItem{}
		if err := rows.Scan(&it.ID, &it.ConfigKey, &it.ConfigValue, &it.Remark, &it.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// UpsertProductConfig 写入或更新指定产品库的配置项
func (r *SettingRepository) UpsertProductConfig(dbName, key, value, remark string) error {
	if !allowedProductDBs[dbName] {
		return fmt.Errorf("不支持的产品库: %s", dbName)
	}
	_, err := r.db.Exec(`INSERT INTO `+dbName+`.product_config (config_key, config_value, remark, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())
		ON DUPLICATE KEY UPDATE config_value = VALUES(config_value), remark = VALUES(remark), updated_at = NOW()`,
		key, value, remark)
	return err
}

// InsertProductConfigIfAbsent 仅在该键不存在时写入产品库配置（启动时预置目录，不覆盖已改的值）
func (r *SettingRepository) InsertProductConfigIfAbsent(dbName, key, value, remark string) error {
	if !allowedProductDBs[dbName] {
		return fmt.Errorf("不支持的产品库: %s", dbName)
	}
	_, err := r.db.Exec(`INSERT IGNORE INTO `+dbName+`.product_config (config_key, config_value, remark, created_at, updated_at)
		VALUES (?, ?, ?, NOW(), NOW())`, key, value, remark)
	return err
}

// DeleteProductConfig 删除产品库配置项（删除后该键回落代码内置默认值）
func (r *SettingRepository) DeleteProductConfig(dbName, key string) error {
	if !allowedProductDBs[dbName] {
		return fmt.Errorf("不支持的产品库: %s", dbName)
	}
	_, err := r.db.Exec(`DELETE FROM `+dbName+`.product_config WHERE config_key = ?`, key)
	return err
}
