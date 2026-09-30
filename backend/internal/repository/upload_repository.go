package repository

import (
	"database/sql"
	"time"

	"oemrpa/internal/model"
)

// UploadRepository 用户上传文件登记与归属校验（图片受 UploadAuth 保护，需按归属放行）
type UploadRepository struct {
	db *sql.DB
}

func NewUploadRepository(db *sql.DB) *UploadRepository {
	return &UploadRepository{db: db}
}

// Create 登记一次上传（同一张图被不同用户上传时各登记一行，用于归属校验）
func (r *UploadRepository) Create(userID int64, filePath, fileURL string, fileSize int64) error {
	_, err := r.db.Exec(
		`INSERT INTO `+model.SysDB+`.upload_file (user_id, file_path, file_url, file_size, created_at)
			VALUES (?, ?, ?, ?, ?)`,
		userID, filePath, fileURL, fileSize, time.Now(),
	)
	return err
}

// OwnedBy 判断该图片是否由指定用户上传
func (r *UploadRepository) OwnedBy(userID int64, filePath string) (bool, error) {
	var n int
	err := r.db.QueryRow(
		`SELECT COUNT(*) FROM `+model.SysDB+`.upload_file WHERE user_id = ? AND file_path = ?`,
		userID, filePath,
	).Scan(&n)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// UploadRow 后台上传文件列表行（含归属用户信息）
type UploadRow struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Phone     string    `json:"phone"`
	Username  string    `json:"username"`
	FilePath  string    `json:"file_path"`
	FileURL   string    `json:"file_url"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

// ListPage 后台分页查询上传文件登记（filePath 为路径模糊匹配）
func (r *UploadRepository) ListPage(userID *int64, filePath, startDate, endDate string, page, pageSize int) ([]*UploadRow, int64, error) {
	where := "1=1"
	args := []interface{}{}
	if userID != nil {
		where += " AND f.user_id = ?"
		args = append(args, *userID)
	}
	if filePath != "" {
		where += " AND f.file_path LIKE ?"
		args = append(args, "%"+filePath+"%")
	}
	if startDate != "" {
		where += " AND f.created_at >= ?"
		args = append(args, startDate+" 00:00:00")
	}
	if endDate != "" {
		where += " AND f.created_at <= ?"
		args = append(args, endDate+" 23:59:59")
	}

	var total int64
	if err := r.db.QueryRow("SELECT COUNT(*) FROM "+model.SysDB+".upload_file f WHERE "+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT f.id, f.user_id, COALESCE(u.phone,''), COALESCE(u.username,''),
			f.file_path, f.file_url, f.file_size, f.created_at
		FROM ` + model.SysDB + `.upload_file f
		LEFT JOIN ` + model.SysDB + `.user u ON u.id = f.user_id
		WHERE ` + where + ` ORDER BY f.id DESC LIMIT ? OFFSET ?`
	rows, err := r.db.Query(query, append(append([]interface{}{}, args...), pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	list := make([]*UploadRow, 0, pageSize)
	for rows.Next() {
		row := &UploadRow{}
		if err := rows.Scan(&row.ID, &row.UserID, &row.Phone, &row.Username, &row.FilePath, &row.FileURL,
			&row.FileSize, &row.CreatedAt); err != nil {
			return nil, 0, err
		}
		list = append(list, row)
	}
	return list, total, rows.Err()
}
