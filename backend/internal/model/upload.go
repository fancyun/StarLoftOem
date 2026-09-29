package model

import "time"

// UploadFile 用户上传文件登记（营业执照/身份证/授权书等资质图）。
// 图片按内容 MD5 命名，同一张图被不同用户上传时各自登记一行，
// 用于控制台用户凭自己的登录态读取本人上传的图片。
func (UploadFile) TableName() string { return SysDB + ".upload_file" }

type UploadFile struct {
	ID        int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID    int64     `json:"user_id" gorm:"not null;default:0;index:idx_user_path"`
	FilePath  string    `json:"file_path" gorm:"size:255;not null;index:idx_user_path"` // 相对路径 yyyyMMdd/{md5}.jpg
	FileURL   string    `json:"file_url" gorm:"size:512"`                               // 对外图片地址
	FileSize  int64     `json:"file_size" gorm:"not null;default:0"`                    // 字节数
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}
