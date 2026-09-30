package handler

import (
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/repository"
	"oemrpa/internal/utils"
)

// logFileNames 后台可查看的日志文件白名单（固定文件名，接口不接受任意路径，防目录穿越）
var logFileNames = []string{"access.log", "admin.log", "business.log", "error.log", "syscall.log"}

// AdminLogHandler 后台日志与审计：登录日志（用户/管理员）与系统日志文件查看
type AdminLogHandler struct {
	loginLogRepo *repository.LoginLogRepository
	logDir       string
}

func NewAdminLogHandler(repo *repository.LoginLogRepository, logDir string) *AdminLogHandler {
	if logDir == "" {
		logDir = "/app/logs"
	}
	return &AdminLogHandler{loginLogRepo: repo, logDir: logDir}
}

// ListUserLoginLogs 后台查询用户登录日志
// GET /admin/login-logs/users?keyword=&status=&start_date=&end_date=&page=&page_size=
func (h *AdminLogHandler) ListUserLoginLogs(c *gin.Context) {
	page, pageSize := paginationParams(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := queryStatus(c)
	startDate, endDate := queryDateRange(c)

	list, total, err := h.loginLogRepo.ListUserLoginLogs(keyword, status, startDate, endDate, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取用户登录日志失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"list": list, "total": total, "page": page, "page_size": pageSize,
	}})
}

// ListAdminLoginLogs 后台查询管理员登录日志
// GET /admin/login-logs/admins?keyword=&status=&start_date=&end_date=&page=&page_size=
func (h *AdminLogHandler) ListAdminLoginLogs(c *gin.Context) {
	page, pageSize := paginationParams(c)
	keyword := strings.TrimSpace(c.Query("keyword"))
	status := queryStatus(c)
	startDate, endDate := queryDateRange(c)

	list, total, err := h.loginLogRepo.ListAdminLoginLogs(keyword, status, startDate, endDate, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取管理员登录日志失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"list": list, "total": total, "page": page, "page_size": pageSize,
	}})
}

// ListLogFiles 列出可查看的日志文件（名称/大小/最后写入时间）
// GET /admin/logs/files
func (h *AdminLogHandler) ListLogFiles(c *gin.Context) {
	files := make([]gin.H, 0, len(logFileNames))
	for _, name := range logFileNames {
		item := gin.H{"name": name, "size_bytes": int64(0), "exists": false}
		if st, err := os.Stat(filepath.Join(h.logDir, name)); err == nil {
			item["size_bytes"] = st.Size()
			item["modified_at"] = st.ModTime().Format(time.RFC3339)
			item["exists"] = true
		}
		files = append(files, item)
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"dir": h.logDir, "files": files}})
}

// TailLogFile 读取指定日志文件的尾部若干行（可选关键词过滤，过滤范围仅本次返回窗口）
// GET /admin/logs/files/:name?lines=200&keyword=
func (h *AdminLogHandler) TailLogFile(c *gin.Context) {
	name := c.Param("name")
	if !isLogFileName(name) {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid log file"})
		return
	}

	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "200"))
	if lines < 1 {
		lines = 200
	}
	if lines > 2000 {
		lines = 2000
	}
	keyword := strings.TrimSpace(c.Query("keyword"))

	path := filepath.Join(h.logDir, name)
	rows, truncated, err := utils.TailFile(path, lines, 0)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusOK, gin.H{"code": 404, "message": "日志文件不存在"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取日志文件失败"})
		return
	}

	matched := 0
	if keyword != "" {
		lower := strings.ToLower(keyword)
		filtered := make([]string, 0, len(rows))
		for _, line := range rows {
			if strings.Contains(strings.ToLower(line), lower) {
				filtered = append(filtered, line)
			}
		}
		matched = len(filtered)
		rows = filtered
	}

	size := int64(0)
	if st, serr := os.Stat(path); serr == nil {
		size = st.Size()
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"name":       name,
		"size_bytes": size,
		"returned":   len(rows),
		"matched":    matched,
		"truncated":  truncated,
		"lines":      rows,
	}})
}

// isLogFileName 校验文件名是否在白名单内
func isLogFileName(name string) bool {
	for _, n := range logFileNames {
		if n == name {
			return true
		}
	}
	return false
}

// queryStatus 解析 status 查询参数（留空表示不限）
func queryStatus(c *gin.Context) *int {
	v := c.Query("status")
	if v == "" {
		return nil
	}
	s, err := strconv.Atoi(v)
	if err != nil {
		return nil
	}
	return &s
}

// queryDateRange 解析日期区间查询参数
func queryDateRange(c *gin.Context) (string, string) {
	return strings.TrimSpace(c.Query("start_date")), strings.TrimSpace(c.Query("end_date"))
}