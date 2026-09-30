package repository

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"oemrpa/internal/model"
)

// SmsResourcePackRepository 短信资源包仓储（操作短信库 oem_sms 下的独立资源包表）。
// product 区分二级类型（sms-验证码/通知短信、sms_marketing-营销短信），两类包互不通用。
type SmsResourcePackRepository struct {
	db *sql.DB
}

func NewSmsResourcePackRepository(db *sql.DB) *SmsResourcePackRepository {
	return &SmsResourcePackRepository{db: db}
}

// ---------- 短信资源包定义 ----------

// CreatePack 创建短信资源包
func (r *SmsResourcePackRepository) CreatePack(pack *model.SmsResourcePack) error {
	query := `INSERT INTO ` + model.SmsDB + `.resource_pack (name, total_count, price, status, product, description, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := r.db.Exec(query,
		pack.Name, pack.TotalCount, pack.Price, pack.Status, pack.Product, pack.Description,
		time.Now(), time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	pack.ID = id
	return nil
}

// UpdatePack 更新短信资源包
func (r *SmsResourcePackRepository) UpdatePack(pack *model.SmsResourcePack) error {
	query := `UPDATE ` + model.SmsDB + `.resource_pack SET name = ?, total_count = ?, price = ?, status = ?, product = ?, description = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query,
		pack.Name, pack.TotalCount, pack.Price, pack.Status, pack.Product, pack.Description,
		time.Now(), pack.ID,
	)
	return err
}

// GetPackByID 根据ID查询短信资源包
func (r *SmsResourcePackRepository) GetPackByID(id int64) (*model.SmsResourcePack, error) {
	query := `SELECT id, name, total_count, price, status, product, description, created_at, updated_at FROM ` + model.SmsDB + `.resource_pack WHERE id = ?`
	pack := &model.SmsResourcePack{}
	err := r.db.QueryRow(query, id).Scan(
		&pack.ID, &pack.Name, &pack.TotalCount, &pack.Price,
		&pack.Status, &pack.Product, &pack.Description, &pack.CreatedAt, &pack.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrPackNotFound
	}
	if err != nil {
		return nil, err
	}
	return pack, nil
}

// ListPacks 查询短信资源包列表（status 为 nil 时查询全部；product 为空时查询全部类型）
func (r *SmsResourcePackRepository) ListPacks(status *int, product string) ([]*model.SmsResourcePack, error) {
	query := `SELECT id, name, total_count, price, status, product, description, created_at, updated_at FROM ` + model.SmsDB + `.resource_pack`
	conds := make([]string, 0, 2)
	args := make([]interface{}, 0, 2)
	if status != nil {
		conds = append(conds, `status = ?`)
		args = append(args, *status)
	}
	if product != "" {
		conds = append(conds, `product = ?`)
		args = append(args, product)
	}
	if len(conds) > 0 {
		query += ` WHERE ` + strings.Join(conds, " AND ")
	}
	query += ` ORDER BY id ASC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	packs := make([]*model.SmsResourcePack, 0)
	for rows.Next() {
		pack := &model.SmsResourcePack{}
		if err := rows.Scan(
			&pack.ID, &pack.Name, &pack.TotalCount, &pack.Price,
			&pack.Status, &pack.Product, &pack.Description, &pack.CreatedAt, &pack.UpdatedAt,
		); err != nil {
			return nil, err
		}
		packs = append(packs, pack)
	}
	return packs, nil
}

// ---------- 用户短信资源包 ----------

// CreateUserPackTx 在事务中创建用户短信资源包
func (r *SmsResourcePackRepository) CreateUserPackTx(tx *sql.Tx, up *model.SmsUserResourcePack) error {
	query := `INSERT INTO ` + model.SmsDB + `.user_resource_pack (user_id, total_count, remaining_count, product, price, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	result, err := tx.Exec(query,
		up.UserID, up.TotalCount, up.RemainingCount, up.Product, up.Price, up.Status,
		time.Now(), time.Now(),
	)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	up.ID = id
	return nil
}

// GetUserActivePack 获取用户指定类型 product 下第一个有效短信资源包（剩余条数 > 0），按购买时间升序
func (r *SmsResourcePackRepository) GetUserActivePack(userID int64, product string) (*model.SmsUserResourcePack, error) {
	query := `SELECT id, user_id, total_count, remaining_count, product, price, status, created_at, updated_at
		FROM ` + model.SmsDB + `.user_resource_pack
		WHERE user_id = ? AND product = ? AND status = 1 AND remaining_count > 0
		ORDER BY created_at ASC
		LIMIT 1`
	up := &model.SmsUserResourcePack{}
	err := r.db.QueryRow(query, userID, product).Scan(
		&up.ID, &up.UserID, &up.TotalCount,
		&up.RemainingCount, &up.Product, &up.Price, &up.Status, &up.CreatedAt, &up.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return up, nil
}

// DeductUserPackCount 扣减用户短信资源包条数（按发送条数 count 原子 UPDATE，未命中返回 false）
func (r *SmsResourcePackRepository) DeductUserPackCount(id, userID int64, count int) (bool, error) {
	if count <= 0 {
		count = 1
	}
	query := `UPDATE ` + model.SmsDB + `.user_resource_pack
		SET remaining_count = remaining_count - ?,
		    status = IF(remaining_count - ? <= 0, 0, 1),
		    updated_at = ?
		WHERE id = ? AND user_id = ? AND remaining_count >= ? AND status = 1`
	result, err := r.db.Exec(query, count, count, time.Now(), id, userID, count)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// RefundUserPackCount 退还短信资源包条数（回执失败等退款场景加回原包）
func (r *SmsResourcePackRepository) RefundUserPackCount(id int64, count int) error {
	if count <= 0 {
		count = 1
	}
	query := `UPDATE ` + model.SmsDB + `.user_resource_pack SET remaining_count = remaining_count + ?, status = 1, updated_at = ? WHERE id = ?`
	res, err := r.db.Exec(query, count, time.Now(), id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("短信资源包不存在: %d", id)
	}
	return nil
}

// RefundUserPackCountByUser 退还短信资源包条数到该用户最近一条资源包（历史记录未记资源包 ID 时的退款兜底）
func (r *SmsResourcePackRepository) RefundUserPackCountByUser(userID int64, count int) error {
	if count <= 0 {
		count = 1
	}
	query := `UPDATE ` + model.SmsDB + `.user_resource_pack SET remaining_count = remaining_count + ?, status = 1, updated_at = ?
		WHERE user_id = ? ORDER BY id DESC LIMIT 1`
	res, err := r.db.Exec(query, count, time.Now(), userID)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil {
		return err
	} else if n == 0 {
		return fmt.Errorf("用户无短信资源包可退回: user_id=%d", userID)
	}
	return nil
}

// ListUserPacks 查询用户全部短信资源包（含已耗尽），按购买时间倒序
func (r *SmsResourcePackRepository) ListUserPacks(userID int64) ([]*model.SmsUserResourcePack, error) {
	query := `SELECT id, user_id, total_count, remaining_count, product, price, status, created_at, updated_at
		FROM ` + model.SmsDB + `.user_resource_pack
		WHERE user_id = ?
		ORDER BY id DESC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*model.SmsUserResourcePack, 0)
	for rows.Next() {
		up := &model.SmsUserResourcePack{}
		if err := rows.Scan(
			&up.ID, &up.UserID, &up.TotalCount,
			&up.RemainingCount, &up.Product, &up.Price, &up.Status, &up.CreatedAt, &up.UpdatedAt,
		); err != nil {
			return nil, err
		}
		list = append(list, up)
	}
	return list, nil
}
