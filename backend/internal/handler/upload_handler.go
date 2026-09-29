package handler

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"oemrpa/internal/config"
	"oemrpa/internal/repository"
	"oemrpa/internal/site"
	"oemrpa/internal/utils"

	"github.com/gin-gonic/gin"
)

// UploadHandler 用户文件上传（营业执照/身份证等图片，供签名报备等场景使用）
type UploadHandler struct {
	cfg        *config.Config
	uploadRepo *repository.UploadRepository
	// siteHosts 按上传者归属解析站点域名组：图片对外地址取其中的图片站点域名
	siteHosts func(userID int64) site.Hosts
}

func NewUploadHandler(cfg *config.Config, uploadRepo *repository.UploadRepository, siteHosts func(userID int64) site.Hosts) *UploadHandler {
	return &UploadHandler{cfg: cfg, uploadRepo: uploadRepo, siteHosts: siteHosts}
}

// 对外图片独立媒体子域（平台为 img.starloft.cn，推广 为其图片站点域名），
// 上传图片对外地址为 /uploads/yyyyMMdd/{内容MD5}.jpg?e={过期时间}&s={访问签名}，
// 经 Nginx 反代到后端 /img/uploads/* 校验签名（不限制来源 IP，上游出口 IP 变动不受影响）。
var uploadMaxSize = int64(5 << 20) // 单文件上限 5MB

var uploadAllowedExt = map[string]bool{
	".jpg":  true,
	".jpeg": true,
	".png":  true,
}

// UploadFile 接收 multipart 字段 file，保存到按日期分目录的上传目录，返回可公网访问的图片 URL
func (h *UploadHandler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "未收到上传文件"})
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !uploadAllowedExt[ext] {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "仅支持 jpg/jpeg/png 图片"})
		return
	}
	if header.Size > uploadMaxSize {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "图片大小不能超过 5MB"})
		return
	}

	dateDir := time.Now().Format("20060102")
	dir := filepath.Join(h.cfg.UploadDir, dateDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "上传失败，请稍后重试"})
		return
	}

	// 读入内容并计算 MD5 作为文件名（内容同图去重、不可枚举，保留扩展名便于 Nginx 判定 Content-Type）
	data, err := io.ReadAll(file)
	if err != nil || len(data) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "上传失败，请稍后重试"})
		return
	}
	sum := md5.Sum(data)
	name := hex.EncodeToString(sum[:]) + ext
	dst, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "上传失败，请稍后重试"})
		return
	}
	defer dst.Close()
	if _, err := dst.Write(data); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "上传失败，请稍后重试"})
		return
	}

	uploaderID := c.GetInt64("user_id")
	hosts := site.Platform()
	if h.siteHosts != nil && uploaderID > 0 {
		hosts = h.siteHosts(uploaderID)
	}

	relPath := dateDir + "/" + name
	expireAt := time.Now().AddDate(0, 0, h.cfg.UploadURLTTLDays).Unix()
	url := fmt.Sprintf("%s/uploads/%s/%s?e=%d&s=%s", hosts.ImgBase(), dateDir, name, expireAt, utils.SignUploadPath(relPath, expireAt))

	// 登记图片归属（控制台用户据此读取本人上传的图）；登记失败不阻断，图片已落盘
	if uploaderID > 0 && h.uploadRepo != nil {
		if err := h.uploadRepo.Create(uploaderID, relPath, url, int64(len(data))); err != nil {
			log.Printf("登记上传图片归属失败 [user_id=%d, path=%s]: %v", uploaderID, relPath, err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"url": url}})
}
