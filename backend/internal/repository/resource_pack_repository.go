package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"oemrpa/internal/model"
)

var (
	ErrPackNotFound   = errors.New("resource pack not found")
	ErrPackOffSale    = errors.New("resource pack off sale")
	ErrPackCountEmpty = errors.New("resource pack count empty")
)

type ResourcePackRepository struct {
	db *sql.DB
}

func NewResourcePackRepository(db *sql.DB) *ResourcePackRepository {
	return &ResourcePackRepository{db: db}
}

// ---------- 资源包定义 ----------

// CreatePack 创建资源包
func (r *ResourcePackRepository) CreatePack(pack *model.ResourcePack) error {
	query := `INSERT INTO ` + model.FvDB + `.resource_pack (name, total_count, price, status, product, description, created_at, updated_at)
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

// UpdatePack 更新资源包
func (r *ResourcePackRepository) UpdatePack(pack *model.ResourcePack) error {
	query := `UPDATE ` + model.FvDB + `.resource_pack SET name = ?, total_count = ?, price = ?, status = ?, product = ?, description = ?, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query,
		pack.Name, pack.TotalCount, pack.Price, pack.Status, pack.Product, pack.Description,
		time.Now(), pack.ID,
	)
	return err
}

// GetPackByID 根据ID查询资源包
func (r *ResourcePackRepository) GetPackByID(id int64) (*model.ResourcePack, error) {
	query := `SELECT id, name, total_count, price, status, COALESCE(product, ''), description, created_at, updated_at FROM ` + model.FvDB + `.resource_pack WHERE id = ?`
	pack := &model.ResourcePack{}
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

// ListPacks 查询资源包列表（status 为 nil 时查询全部；否则按状态过滤）
func (r *ResourcePackRepository) ListPacks(status *int) ([]*model.ResourcePack, error) {
	query := `SELECT id, name, total_count, price, status, COALESCE(product, ''), description, created_at, updated_at FROM ` + model.FvDB + `.resource_pack`
	args := make([]interface{}, 0)
	if status != nil {
		query += ` WHERE status = ?`
		args = append(args, *status)
	}
	query += ` ORDER BY id ASC`

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	packs := make([]*model.ResourcePack, 0)
	for rows.Next() {
		pack := &model.ResourcePack{}
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

// ---------- 用户资源包 ----------

// CreateUserPackTx 在事务中创建用户资源包
func (r *ResourcePackRepository) CreateUserPackTx(tx *sql.Tx, up *model.UserResourcePack) error {
	query := `INSERT INTO ` + model.FvDB + `.user_resource_pack (user_id, total_count, remaining_count, product, price, status, created_at, updated_at)
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

// GetUserActivePack 获取用户指定子产品 product 下第一个有效资源包（剩余次数 > 0），按购买时间升序。
// 匹配优先级：精确子产品（如 fv_auth）> 所属一级产品的通用包（如 fv）> 未标注产品的历史包（空）。
// 跨一级产品互不通用：短信等其它产品的资源包存放于各自产品库，不参与此处匹配。
func (r *ResourcePackRepository) GetUserActivePack(userID int64, product string) (*model.UserResourcePack, error) {
	candidates := []string{product}
	switch product {
	case model.ServiceFVAuth, model.ServiceFVSelf:
		// 人脸核验：有源/无源子产品包优先，其次一级产品 fv 通用包，最后兼容早期无产品标记的包
		candidates = append(candidates, model.ProductFV, "")
	}
	in := strings.Repeat(",?", len(candidates))[1:]
	query := `SELECT id, user_id, total_count, remaining_count, COALESCE(product, ''), price, status, created_at, updated_at
		FROM ` + model.FvDB + `.user_resource_pack
		WHERE user_id = ? AND status = 1 AND remaining_count > 0
		  AND COALESCE(product, '') IN (` + in + `)
		ORDER BY FIELD(COALESCE(product, ''), ` + in + `), created_at ASC
		LIMIT 1`
	args := make([]interface{}, 0, 1+len(candidates)*2)
	args = append(args, userID)
	for _, c := range candidates {
		args = append(args, c)
	}
	for _, c := range candidates {
		args = append(args, c)
	}
	up := &model.UserResourcePack{}
	err := r.db.QueryRow(query, args...).Scan(
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

// DeductUserPackCount 扣减用户资源包次数（单条原子 UPDATE，无需事务）
// 返回 false 表示资源包已耗尽/无剩余次数（并发安全，由 remaining_count > 0 条件保证）
func (r *ResourcePackRepository) DeductUserPackCount(id, userID int64) (bool, error) {
	query := `UPDATE ` + model.FvDB + `.user_resource_pack
		SET remaining_count = remaining_count - 1,
		    status = IF(remaining_count - 1 <= 0, 0, 1),
		    updated_at = ?
		WHERE id = ? AND user_id = ? AND remaining_count > 0 AND status = 1`
	result, err := r.db.Exec(query, time.Now(), id, userID)
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected > 0, nil
}

// RefundUserPackCount 退还资源包次数（退款/不计费时加回）
func (r *ResourcePackRepository) RefundUserPackCount(id int64) error {
	query := `UPDATE ` + model.FvDB + `.user_resource_pack SET remaining_count = remaining_count + 1, status = 1, updated_at = ? WHERE id = ?`
	_, err := r.db.Exec(query, time.Now(), id)
	return err
}

// ListUserPacks 查询用户全部资源包（含已耗尽），按购买时间倒序
func (r *ResourcePackRepository) ListUserPacks(userID int64) ([]*model.UserResourcePack, error) {
	query := `SELECT id, user_id, total_count, remaining_count, COALESCE(product, ''), price, status, created_at, updated_at
		FROM ` + model.FvDB + `.user_resource_pack
		WHERE user_id = ?
		ORDER BY id DESC`
	rows, err := r.db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*model.UserResourcePack, 0)
	for rows.Next() {
		up := &model.UserResourcePack{}
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
