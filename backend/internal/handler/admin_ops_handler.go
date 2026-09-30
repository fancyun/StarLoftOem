package handler

import (
	"context"
	"database/sql"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/config"
	"oemrpa/internal/model"
	"oemrpa/internal/redis"
	"oemrpa/internal/repository"
	"oemrpa/internal/service"
	"oemrpa/internal/utils"
)

// AdminOpsHandler 后台运维可观测：系统监控汇总与下游通知重试队列
type AdminOpsHandler struct {
	db         *sql.DB
	notifyRepo *repository.NotifyRecordRepository
	notifySvc  *service.NotifyService
	cfg        *config.Config
}

func NewAdminOpsHandler(
	db *sql.DB,
	notifyRepo *repository.NotifyRecordRepository,
	notifySvc *service.NotifyService,
	cfg *config.Config,
) *AdminOpsHandler {
	return &AdminOpsHandler{db: db, notifyRepo: notifyRepo, notifySvc: notifySvc, cfg: cfg}
}

// GetSystemMonitor 系统监控汇总：数据库/Redis 探针、进程指标、日志文件、库表概览与轻量业务指标
// GET /admin/system/monitor
func (h *AdminOpsHandler) GetSystemMonitor(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	process := gin.H{
		"uptime_seconds":       int64(time.Since(utils.ProcessStartTime).Seconds()),
		"goroutines":           runtime.NumGoroutine(),
		"go_version":           runtime.Version(),
		"mem_alloc_bytes":      m.Alloc,
		"mem_sys_bytes":        m.Sys,
		"mem_heap_inuse_bytes": m.HeapInuse,
		"gc_count":             m.NumGC,
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"database": h.databaseStatus(ctx),
		"redis":    h.redisStatus(ctx),
		"process":  process,
		"logs":     h.logFileStatus(),
		"tables":   h.tableStatus(ctx),
		"config": gin.H{
			"server":     h.cfg.Server.Host + ":" + h.cfg.Server.Port,
			"log_dir":    h.cfg.Log.Dir,
			"upload_dir": h.cfg.UploadDir,
			"media_dir":  h.cfg.MediaDir,
		},
	}})
}

// GetBusinessStats 业务规模指标（后台账号数 / 系统配置项数 / 通知重试积压），展示在「数据统计」页。
// GET /admin/stats/business —— 权限沿用 sys.dashboard（路由前缀 /admin/stats）
func (h *AdminOpsHandler) GetBusinessStats(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	out := gin.H{
		"admins":           0,
		"setting_items":    0,
		"notify_pending":   0,
		"notify_abandoned": 0,
	}
	for _, item := range []struct {
		key   string
		table string
	}{
		{"admins", "admin_user"},
		{"setting_items", "setting"},
	} {
		var n int64
		if err := h.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+model.SysDB+"."+item.table).Scan(&n); err == nil {
			out[item.key] = n
		}
	}
	if h.notifyRepo != nil {
		if counts, err := h.notifyRepo.CountByStatus(); err == nil {
			out["notify_pending"] = counts[model.NotifyPending]
			out["notify_abandoned"] = counts[model.NotifyAbandoned]
		}
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": out})
}

// databaseStatus 数据库探针（Ping 耗时 + 连接池统计）
func (h *AdminOpsHandler) databaseStatus(ctx context.Context) gin.H {
	start := time.Now()
	if err := h.db.PingContext(ctx); err != nil {
		return gin.H{"ok": false, "error": err.Error()}
	}
	st := h.db.Stats()
	return gin.H{
		"ok":                   true,
		"ping_ms":              time.Since(start).Milliseconds(),
		"open_connections":     st.OpenConnections,
		"in_use":               st.InUse,
		"idle":                 st.Idle,
		"wait_count":           st.WaitCount,
		"wait_duration_ms":     st.WaitDuration.Milliseconds(),
		"max_open_connections": st.MaxOpenConnections,
		"max_idle_closed":      st.MaxIdleClosed,
	}
}

// redisStatus Redis 探针（Ping 耗时 + INFO 内存/连接/命中统计 + 客户端连接池）
func (h *AdminOpsHandler) redisStatus(ctx context.Context) gin.H {
	if redis.Client == nil {
		return gin.H{"ok": false, "error": "redis 未配置"}
	}
	start := time.Now()
	if err := redis.Client.Ping(ctx).Err(); err != nil {
		return gin.H{"ok": false, "error": err.Error()}
	}
	out := gin.H{"ok": true, "ping_ms": time.Since(start).Milliseconds()}
	if text, err := redis.Client.Info(ctx).Result(); err == nil {
		parseRedisInfo(text, out)
	}
	if pool := redis.Client.PoolStats(); pool != nil {
		out["pool"] = gin.H{
			"total_conns": pool.TotalConns,
			"idle_conns":  pool.IdleConns,
			"stale_conns": pool.StaleConns,
			"hits":        pool.Hits,
			"misses":      pool.Misses,
			"timeouts":    pool.Timeouts,
		}
	}
	return out
}

// parseRedisInfo 解析 INFO 文本中的关键指标（内存/连接/命中/键空间）
func parseRedisInfo(text string, out gin.H) {
	keyspace := gin.H{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch k {
		case "used_memory":
			out["used_memory_bytes"] = parseInt64(v)
		case "connected_clients":
			out["connected_clients"] = parseInt64(v)
		case "keyspace_hits":
			out["keyspace_hits"] = parseInt64(v)
		case "keyspace_misses":
			out["keyspace_misses"] = parseInt64(v)
		case "instantaneous_ops_per_sec":
			out["ops_per_sec"] = parseInt64(v)
		default:
			// 键空间行形如 db0:keys=12,expires=3,avg_ttl=0
			if strings.HasPrefix(k, "db") && strings.Contains(v, "keys=") {
				item := gin.H{}
				for _, seg := range strings.Split(v, ",") {
					sk, sv, sok := strings.Cut(seg, "=")
					if sok {
						item[sk] = parseInt64(sv)
					}
				}
				keyspace[k] = item
			}
		}
	}
	if len(keyspace) > 0 {
		out["keyspace"] = keyspace
	}
}

// logFileStatus 各类日志文件的大小与最后写入时间
func (h *AdminOpsHandler) logFileStatus() []gin.H {
	dir := h.cfg.Log.Dir
	if dir == "" {
		dir = "/app/logs"
	}
	files := make([]gin.H, 0, len(logFileNames))
	for _, name := range logFileNames {
		item := gin.H{"name": name, "size_bytes": int64(0), "exists": false}
		if st, err := os.Stat(filepath.Join(dir, name)); err == nil {
			item["size_bytes"] = st.Size()
			item["modified_at"] = st.ModTime().Format(time.RFC3339)
			item["exists"] = true
		}
		files = append(files, item)
	}
	return files
}

// tableStatus 库表概览：三库表数量 + 近似行数 Top 20（information_schema，不扫表）
func (h *AdminOpsHandler) tableStatus(ctx context.Context) gin.H {
	schemas := "'" + model.SysDB + "','" + model.FvDB + "','" + model.SmsDB + "'"
	raw := gin.H{}
	if rows, err := h.db.QueryContext(ctx,
		`SELECT TABLE_SCHEMA, COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA IN (`+schemas+`) GROUP BY TABLE_SCHEMA`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var schema string
			var count int64
			if rows.Scan(&schema, &count) == nil {
				raw[schema] = count
			}
		}
	}

	top := make([]gin.H, 0, 20)
	if rows, err := h.db.QueryContext(ctx,
		`SELECT TABLE_SCHEMA, TABLE_NAME, TABLE_ROWS FROM information_schema.TABLES
			WHERE TABLE_SCHEMA IN (`+schemas+`) AND TABLE_ROWS IS NOT NULL
			ORDER BY TABLE_ROWS DESC LIMIT 20`); err == nil {
		defer rows.Close()
		for rows.Next() {
			var schema, table string
			var rowsCount int64
			if rows.Scan(&schema, &table, &rowsCount) == nil {
				top = append(top, gin.H{"schema": schema, "table": table, "rows": rowsCount})
			}
		}
	}
	return gin.H{"counts": raw, "top_rows": top}
}

// ListNotifyRecords 后台分页查询下游通知重试记录
// GET /admin/notify-records?status=&biz_type=&keyword=&start_date=&end_date=&page=&page_size=
func (h *AdminOpsHandler) ListNotifyRecords(c *gin.Context) {
	page, pageSize := paginationParams(c)
	status := queryStatus(c)
	startDate, endDate := queryDateRange(c)

	list, total, err := h.notifyRepo.ListRecords(status, strings.TrimSpace(c.Query("biz_type")),
		strings.TrimSpace(c.Query("keyword")), startDate, endDate, page, pageSize)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": "读取通知重试记录失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{
		"list": list, "total": total, "page": page, "page_size": pageSize,
	}})
}

// RetryNotifyRecord 手动立即重推一条通知（force=true 时把已放弃的记录重置为待推后再推）。
// 记录以「业务类型 + 业务单号」为键，故三者均经请求体传入。
// POST /admin/notify-records/retry  body: {"biz_type":"fv_result","biz_no":"...","force":false}
func (h *AdminOpsHandler) RetryNotifyRecord(c *gin.Context) {
	var req struct {
		BizType string `json:"biz_type" binding:"required"`
		BizNo   string `json:"biz_no" binding:"required"`
		Force   bool   `json:"force"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": "invalid request parameters"})
		return
	}

	success, rerr := h.notifySvc.RetryNow(req.BizType, req.BizNo, req.Force)
	result := "success"
	if rerr != nil {
		result = rerr.Error()
	}
	utils.AdminLogger.Printf("admin_id=%d operation=notify_retry resource_type=notify_record biz_type=%s biz_no=%s force=%v ip=%s result=%s",
		c.GetInt64("admin_id"), req.BizType, req.BizNo, req.Force, c.ClientIP(), result)

	if rerr != nil {
		c.JSON(http.StatusOK, gin.H{"code": 400, "message": rerr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": gin.H{"success": success}})
}

// parseInt64 宽松解析 INFO 中的数值字段（非法或缺失返回 0）
func parseInt64(v string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
