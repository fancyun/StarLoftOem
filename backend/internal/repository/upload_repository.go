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
