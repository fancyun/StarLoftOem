package middleware

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/logstore"
	"oemrpa/internal/utils"
)

// CORSMiddleware 跨域资源共享中间件
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 在生产环境应该配置具体的域名，不要使用 *
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key, X-Sign, X-Sign-Version, X-Timestamp")
			c.Header("Access-Control-Expose-Headers", "Content-Length, X-RateLimit-Limit, X-RateLimit-Remaining")
			c.Header("Access-Control-Max-Age", "86400")
		}

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// Recovery 从panic恢复的中间件
func Recovery() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				utils.ErrorLogger.Printf("Panic recovered: %v", err)
				c.JSON(500, gin.H{
					"code":    500,
					"message": "internal server error",
				})
				c.Abort()
			}
		}()
		c.Next()
	}
}

// RequestLogger 请求日志中间件（按类别写入 ./logs 文件）
// 覆盖：用户操作（console→access.log）、平台 API 调用（api→access.log，经 api.starloft.cn /v1）、
// 上游回调（callback→syscall.log）、管理操作（admin→access.log）。
// 所有访问、调用、操作均在此统一按类别写入本地日志文件。
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		method := c.Request.Method

		c.Next()

		latency := time.Since(start)
		statusCode := c.Writer.Status()
		category := categorizeAccess(path)

		// 识别已登录用户/管理员身份（JWT 中间件写入 user_id /event API 签核写入 user_id）
		var userID, adminID int64
		if id, ok := c.Get("user_id"); ok {
			if v, ok2 := id.(int64); ok2 && v > 0 {
				if category == "admin" {
					adminID = v
				} else {
					userID = v
				}
			}
		}

		// 敏感路径（登录/注册）不记录客户端 IP，保护隐私
		ip := c.ClientIP()
		if isSensitivePath(path) {
			ip = ""
		}

		switch category {
		case "console", "admin", "api":
			serviceCode := ""
			if category == "api" {
				if v, ok := c.Get("service_code"); ok {
					if s, ok2 := v.(string); ok2 {
						serviceCode = s
					}
				}
				if serviceCode == "" {
					serviceCode = serviceFromPath(path)
				}
			}
			logstore.RecordAccess(category, userID, adminID, method, path, statusCode, latency.Milliseconds(), ip, serviceCode)
		case "callback":
			// 上游支付/实名回调：按渠道记入 sys_call_log（失败不入访问表）
			logstore.RecordSysCall("callback", callbackChannel(path), "", 0, "",
				"", fmt.Sprintf("%s %s - %d, ip=%s", method, path, statusCode, ip),
				boolToInt(statusCode == 200))
		}
	}
}

// categorizeAccess 依据路径前缀划分访问类别（后端一级段取访问子域标签：console/admin/img/api）
func categorizeAccess(path string) string {
	switch {
	case strings.HasPrefix(path, "/admin/"):
		return "admin"
	case strings.HasPrefix(path, "/console/"):
		return "console"
	case strings.HasPrefix(path, "/api/v1/callback/"):
		return "callback"
	case strings.HasPrefix(path, "/api/v1/"):
		return "api"
	default:
		return "other"
	}
}

// callbackChannel 从回调路径末段解析第三方渠道标识（如 /api/v1/callback/finauth → finauth）
func callbackChannel(path string) string {
	trimmed := strings.TrimRight(path, "/")
	idx := strings.LastIndex(trimmed, "/")
	if idx >= 0 && idx < len(trimmed)-1 {
		return trimmed[idx+1:]
	}
	return "unknown"
}

// serviceFromPath 依据 API 路径推导所属大类服务标识（当 ServiceAccessGuard 未写入 service_code 时的兜底）
func serviceFromPath(path string) string {
	switch {
	case strings.HasPrefix(path, "/api/v1/sms/"):
		return "sms"
	case strings.HasPrefix(path, "/api/v1/fv/"):
		return "fv"
	default:
		return ""
	}
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func isSensitivePath(path string) bool {
	sensitivePaths := []string{"/console/login", "/admin/login", "/console/register"}
	for _, sp := range sensitivePaths {
		if path == sp {
			return true
		}
	}
	return false
}
