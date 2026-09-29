package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/model"
)

// ServiceAccessGuard 校验 API Key 对指定端点的访问资格（与 APIKeyMiddleware 配合，需在其之后挂载）：
//   - 权限范围：密钥 permission 为 "all" 或包含该端点标识 endpointCode 才放行（权限可逗号分隔多个端点）
//   - 实名等级：所属用户实名状态须达到该端点要求的等级（model.EndpointRealname）
func ServiceAccessGuard(endpointCode string) gin.HandlerFunc {
	needRealname := model.EndpointRealname(endpointCode)
	return func(c *gin.Context) {
		credVal, ok := c.Get("api")
		if !ok {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code":    403,
				"message": "API Key 权限校验失败",
			})
			return
		}
		cred := credVal.(*model.ApiKey)
		if !model.APIAllows(cred.Permission, endpointCode) {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code":    403,
				"message": "API Key 无权访问该接口",
			})
			return
		}
		// 记录本次调用的端点标识，供日志按接口计量与审计
		c.Set("service_code", endpointCode)

		userVal, ok := c.Get("user")
		if !ok {
			c.AbortWithStatusJSON(http.StatusOK, gin.H{
				"code":    403,
				"message": "用户信息获取失败",
			})
			return
		}
		user := userVal.(*model.User)
		if user.RealnameStatus < needRealname {
			if needRealname == model.RealnameEnterprise {
				c.AbortWithStatusJSON(http.StatusOK, gin.H{
					"code":    403,
					"message": "该服务需完成企业实名认证后使用",
				})
			} else {
				c.AbortWithStatusJSON(http.StatusOK, gin.H{
					"code":    403,
					"message": "该服务需完成实名认证后使用",
				})
			}
			return
		}

		c.Next()
	}
}
