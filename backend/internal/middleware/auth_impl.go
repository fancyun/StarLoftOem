package middleware

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/repository"
	"oemrpa/internal/utils"
)

// APIKeyMiddleware API Key authentication middleware
// 校验平台用户（user）的 API Key 与请求签名，密钥对存放于 sys 库 api 表
func APIKeyMiddleware(userRepo *repository.UserRepository, apiKeyRepo *repository.ApiKeyRepository, signMgr *utils.SignatureManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get API Key and signature info from Header
		apiKey := c.GetHeader("X-Api-Key")
		sign := c.GetHeader("X-Sign")
		signVersion := c.GetHeader("X-Sign-Version")
		timestamp := c.GetHeader("X-Timestamp")

		if apiKey == "" || sign == "" || signVersion == "" || timestamp == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "missing required headers",
			})
			c.Abort()
			return
		}

		// Verify timestamp (prevent replay, allow ±5 minutes)
		var ts int64
		_, err := fmt.Sscanf(timestamp, "%d", &ts)
		if err != nil || !signMgr.VerifyTimestamp(ts, 300) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid or expired timestamp",
			})
			c.Abort()
			return
		}

		// Read request body
		bodyBytes, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "failed to read request body",
			})
			c.Abort()
			return
		}

		// Restore request body (for subsequent Handler use)
		c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

		// Verify signature
		if signVersion != "hmac_sha256" {
			c.JSON(http.StatusBadRequest, gin.H{
				"code":    400,
				"message": "unsupported sign version",
			})
			c.Abort()
			return
		}

		// 按 API Key 查询密钥记录并定位所属用户
		cred, err := apiKeyRepo.GetByAPIKey(apiKey)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid api key",
			})
			c.Abort()
			return
		}

		user, err := userRepo.GetUserByID(cred.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid api key",
			})
			c.Abort()
			return
		}

		// Check user status
		if user.Status == 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"code":    403,
				"message": "user disabled",
			})
			c.Abort()
			return
		}

		if !signMgr.VerifyHMACSHA256(cred.APISecret, string(bodyBytes), sign) {
			c.JSON(http.StatusUnauthorized, gin.H{
				"code":    401,
				"message": "invalid signature",
			})
			c.Abort()
			return
		}

		// Store user & api info in context
		c.Set("user_id", user.ID)
		c.Set("user", user)
		c.Set("api", cred)

		c.Next()
	}
}
