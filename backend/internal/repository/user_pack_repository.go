package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
)

// UserPackRepository 用户已购资源包查询（FV 与 SMS 分属两库，此处只读合并展示）
type UserPackRepository struct {
	db *sql.DB
}

func NewUserPackRepository(db *sql.DB) *UserPackRepository {
	return &UserPackRepository{db: db}
}

// UserPackRow 用户已购资源包列表行（含归属用户信息与物理库归属）
type UserPackRow struct {
	ID             int64     `json:"id"`
	UserID         int64     `json:"user_id"`
	Phone          string    `json:"phone"`
	Username       string    `json:"username"`
	PackName       string    `json:"pack_name"`
	TotalCount     int       `json:"total_count"`
	RemainingCount int       `json:"remaining_count"`
	Product        string    `json:"product"`
	Price          float64   `json:"price"`
	Status         int       `json:"status"`
	Scope          string    `json:"scope"` // fv | sms（物理库归属）
	CreatedAt      time.Time `json:"created_at"`
}

// ListPage 后台分页查询用户已购资源包；scope 为空表示合并 fv/sms 两库（同实例跨库 UNION ALL）
func (r *UserPackRepository) ListPage(scope string, userID *int64, status *int, startDate, endDate string, page, pageSize int) ([]*UserPackRow, int64, error) {
	union := `SELECT p.id, p.user_id, COALESCE(u.phone,'') AS phone, COALESCE(u.username,'') AS username,
			p.pack_name, p.total_count, p.remaining_count, COALESCE(p.product,'') AS product,
			p.price, p.status, p.created_at, 'fv' AS scope
		FROM ` + model.FvDB + `.user_resource_pack p
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = p.user_id
		UNION ALL
		SELECT p.id, p.user_id, COALESCE(u.phone,''), COALESCE(u.username,''),
			p.pack_name, p.total_count, p.remaining_count, p.product,
			p.price, p.status, p.created_at, 'sms'
		FROM ` + model.SmsDB + `.user_resource_pack p
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = p.user_id`

	where := "1=1"
	args := []interface{}{}
	if scope != "" {
		where += " AND t.scope = ?"
		args = append(args, scope)
	}
	if userID != nil {
		where += " AND t.user_id = ?"
		args = append(args, *userID)
	}
	if status != nil {
		where += " AND t.status = ?"
		args = append(args, *status)
	}
	if startDate != "" {
		where += " AND t.created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND t.created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	if err := r.db.QueryRow("SELECT COUNT(*) FROM ("+union+") t WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT t.id, t.user_id, t.phone, t.username, t.pack_name, t.total_count, t.remaining_count,
			t.product, t.price, t.status, t.created_at, t.scope
		FROM (` + union + `) t WHERE ` + where + ` ORDER BY t.created_at DESC, t.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*UserPackRow, 0, pageSize)
	for rows.Next() {
		row := &UserPackRow{}
		if err := rows.Scan(&row.ID, &row.UserID, &row.Phone, &row.Username, &row.PackName, &row.TotalCount,
			&row.RemainingCount, &row.Product, &row.Price, &row.Status, &row.CreatedAt, &row.Scope); err != nil {
			return nil, 0, err
		}
		list = append(list, row)
	}
	return list, total, rows.Err()
}