package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/repository"
)

// AdminAssetHandler 后台用户资产与密钥：用户已购资源包、API 密钥、上传文件（均只读）
type AdminAssetHandler struct {
	userPackRepo *repository.UserPackRepository
	apiKeyRepo   *repository.ApiKeyRepository
	uploadRepo   *repository.UploadRepository
}

func NewAdminAssetHandler(
	userPackRepo *repository.UserPackRepository,
	apiKeyRepo *repository.ApiKeyRepository,
	uploadRepo *repository.UploadRepository,
) *AdminAssetHandler {
	return &AdminAssetHandler{userPackRepo: userPackRepo, apiKeyRepo: apiKeyRepo, uploadRepo: uploadRepo}
}

// ListUserPacks 用户已购资源包（scope=fv|sms，留空合并两库）
// GET /admin/user-packs?scope=&user_id=&status=&start_date=&end_date=&page=&page_size=
func (h *AdminAssetHandler) ListUserPacks(c *gin.Context) {
	page, pageSize := paginationParams(c)
	scope := strings.TrimSpace(c.Query("scope"))
	if scope != "" && scope != "fv" && scope != "sms" {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid scope"})
		return
	}
	startDate, endDate := queryDateRange(c)

	list, total, err := h.userPackRepo.ListPage(scope, queryUserID(c), queryStatus(c), startDate, endDate, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取用户资源包失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"list": list, "total": total, "page": page, "page_size": pageSize,
	}})
}

// ListAPIKeys 全平台 API 密钥（只读，不含 api_secret）
// GET /admin/api-keys?user_id=&keyword=&start_date=&end_date=&page=&page_size=
func (h *AdminAssetHandler) ListAPIKeys(c *gin.Context) {
	page, pageSize := paginationParams(c)
	startDate, endDate := queryDateRange(c)

	list, total, err := h.apiKeyRepo.ListPage(queryUserID(c), strings.TrimSpace(c.Query("keyword")),
		startDate, endDate, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取 API 密钥失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"list": list, "total": total, "page": page, "page_size": pageSize,
	}})
}

// ListUploads 上传文件登记（图片预览复用受保护的 /img/uploads/*）
// GET /admin/uploads?user_id=&file_path=&start_date=&end_date=&page=&page_size=
func (h *AdminAssetHandler) ListUploads(c *gin.Context) {
	page, pageSize := paginationParams(c)
	startDate, endDate := queryDateRange(c)

	list, total, err := h.uploadRepo.ListPage(queryUserID(c), strings.TrimSpace(c.Query("file_path")),
		startDate, endDate, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取上传文件失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"list": list, "total": total, "page": page, "page_size": pageSize,
	}})
}

// queryUserID 解析 user_id 查询参数（留空或非法表示不限）
func queryUserID(c *gin.Context) *int64 {
	v := strings.TrimSpace(c.Query("user_id"))
	if v == "" {
		return nil
	}
	id, err := strconv.ParseInt(v, 10, 64)
	if err != nil || id <= 0 {
		return nil
	}
	return &id
}