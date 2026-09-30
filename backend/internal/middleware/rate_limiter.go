package middleware

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/redis"
)

const (
	// API 调用频率限制前缀
	APIRateLimitPrefix = "rate:api:"
	// 默认限制：每分钟60次
	DefaultRateLimit = 60
	// 限制时间窗口
	RateLimitWindow = 60 * time.Second
)

// RateLimiter API 频率限制中间件
func RateLimiter(limit int) gin.HandlerFunc {
	if limit <= 0 {
		limit = DefaultRateLimit
	}

	return func(c *gin.Context) {
		// 获取用户标识（优先使用用户ID，其次使用IP）
		userID, exists := c.Get("user_id")
		var identifier string
		if exists {
			identifier = fmt.Sprintf("user:%v", userID)
		} else {
			identifier = fmt.Sprintf("ip:%s", c.ClientIP())
		}

		// 构造 Redis 键
		key := APIRateLimitPrefix + identifier

		// 增加计数
		count, err := redis.Incr(key)
		if err != nil {
			// Redis 不可用时 fail-closed：拒绝放行，避免限流被绕过
			log.Printf("限流 Redis 异常，拒绝请求: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"message": "服务暂时不可用，请稍后再试",
			})
			c.Abort()
			return
		}

		// 如果是第一次计数，设置过期时间
		if count == 1 {
			_ = redis.Expire(key, RateLimitWindow)
		}

		// 检查是否超过限制
		if count > int64(limit) {
			// 获取剩余时间
			ttl, _ := redis.TTL(key)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":        429,
				"message":     "请求过于频繁，请稍后再试",
				"retry_after": int(ttl.Seconds()),
			})
			c.Abort()
			return
		}

		// 设置响应头
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-int(count)))

		c.Next()
	}
}

// RateLimiterForIP 针对 IP 的频率限制（用于登录、注册等接口）
func RateLimiterForIP(limit int) gin.HandlerFunc {
	return rateLimiterForIP("", limit)
}

// RateLimiterForIPKey 针对 IP + 业务域的频率限制：各业务域使用独立计数键。
// 轮询等高频接口若与 RateLimiterForIP 共用键（rate:api:ip:<ip>）会消耗同一 IP 的共享预算，
// 导致同 IP 的登录/发码被限流误伤。
func RateLimiterForIPKey(scope string, limit int) gin.HandlerFunc {
	return rateLimiterForIP(scope, limit)
}

// rateLimiterForIP 按 IP 限流：scope 为空使用共享键 rate:api:ip:<ip>，非空使用独立键 rate:api:<scope>:ip:<ip>
func rateLimiterForIP(scope string, limit int) gin.HandlerFunc {
	if limit <= 0 {
		limit = DefaultRateLimit
	}

	return func(c *gin.Context) {
		ip := c.ClientIP()
		key := APIRateLimitPrefix + "ip:" + ip
		if scope != "" {
			key = APIRateLimitPrefix + scope + ":ip:" + ip
		}

		count, err := redis.Incr(key)
		if err != nil {
			// Redis 不可用时 fail-closed：拒绝放行，避免登录/注册限流被绕过
			log.Printf("限流 Redis 异常，拒绝请求: %v", err)
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"code":    503,
				"message": "服务暂时不可用，请稍后再试",
			})
			c.Abort()
			return
		}

		if count == 1 {
			_ = redis.Expire(key, RateLimitWindow)
		}

		if count > int64(limit) {
			ttl, _ := redis.TTL(key)
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code":        429,
				"message":     "请求过于频繁，请稍后再试",
				"retry_after": int(ttl.Seconds()),
			})
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", limit-int(count)))

		c.Next()
	}
}
