package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"oemrpa/internal/repository"
	"oemrpa/internal/utils"
)

// UploadAuth 保护上传图片静态访问（后端一级段取访问子域标签：/img/uploads/*）。
// 营业执照/身份证等资质图为敏感资料，按以下顺序判定，命中其一即放行：
//  1. URL 携带有效访问签名（?e=过期时间&s=签名，不限制来源 IP）——对外图片地址的主通道，
//     上游按地址拉图与本平台页面展示均走此通道，上游出口 IP 变动不再导致拉图失败
//  2. 已登录管理员（admin JWT）：可读全部图片（签名过期的图片也可读）
//  3. 已登录控制台用户（user JWT）：仅可读本人上传的图片（按 upload_file 登记归属校验）
//
// 其余来源一律 403。
func UploadAuth(adminSecret, userSecret string, uploadRepo *repository.UploadRepository) gin.HandlerFunc {
	adminJWT := utils.NewJWTManager(adminSecret)
	userJWT := utils.NewJWTManager(userSecret)
	return func(c *gin.Context) {
		filePath, hasPath := uploadRelPath(c.Request.URL.Path)

		// 1. 有效访问签名放行
		if hasPath && utils.VerifyUploadSign(filePath, c.Query("e"), c.Query("s")) {
			c.Next()
			return
		}

		token := bearerToken(c.GetHeader("Authorization"))
		// 2. 已登录管理员放行
		if claims, err := adminJWT.ValidateToken(token); err == nil && claims != nil && claims.UserType == "admin" {
			c.Next()
			return
		}
		// 3. 控制台用户：仅放行本人上传的图片
		if hasPath && uploadRepo != nil {
			if claims, err := userJWT.ValidateToken(token); err == nil && claims != nil && claims.UserType == "user" {
				if owned, err := uploadRepo.OwnedBy(claims.UserID, filePath); err == nil && owned {
					c.Next()
					return
				}
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden"})
	}
}

// uploadRelPath 从静态访问路径中提取上传目录下的相对路径（如 /img/uploads/20260920/xxx.jpg → 20260920/xxx.jpg）
func uploadRelPath(path string) (string, bool) {
	const marker = "/uploads/"
	i := strings.Index(path, marker)
	if i < 0 {
		return "", false
	}
	rel := strings.TrimPrefix(path[i+len(marker):], "/")
	if rel == "" || strings.Contains(rel, "..") {
		return "", false
	}
	return rel, true
}

// bearerToken 从 Authorization 头提取 Bearer token
func bearerToken(header string) string {
	p := strings.SplitN(header, " ", 2)
	if len(p) == 2 && strings.EqualFold(p[0], "Bearer") {
		return strings.TrimSpace(p[1])
	}
	return ""
}
