package middleware

import (
	"log"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// CallbackGuard 保护上游异步回调端点（/v1/callback/*）：
// 回调来自第三方服务器（联麓/FinAuth/支付宝/微信）且通常无签名，为防止伪造推送
// （伪造回执/签名模板状态/支付通知），按来源 IP 白名单校验。
//   - 未配置 CALLBACK_TRUST_IPS 时放行（兼容现状，仅记一次提示日志）；
//   - 配置后仅放行：精确 IP / CIDR 命中 + 环回地址（127.0.0.1/8、::1，内网 Nginx 反代场景）；
//   - 其余来源 403。真实 IP 优先取 X-Forwarded-For 最后一跳（需 Nginx 反代透传该头）。
func CallbackGuard(trustIPs []string) gin.HandlerFunc {
	nets := make([]*net.IPNet, 0, len(trustIPs))
	exact := make(map[string]struct{}, len(trustIPs))
	for _, s := range trustIPs {
		if ip := net.ParseIP(s); ip != nil {
			exact[ip.String()] = struct{}{}
			continue
		}
		if _, ipnet, err := net.ParseCIDR(s); err == nil {
			nets = append(nets, ipnet)
		}
	}
	if len(exact) == 0 && len(nets) == 0 {
		log.Println("CALLBACK_TRUST_IPS 未配置，回调端点暂不校验来源（建议配置上游推送 IP 白名单）")
	}
	return func(c *gin.Context) {
		// 未配置白名单：放行
		if len(exact) == 0 && len(nets) == 0 {
			c.Next()
			return
		}
		ip := clientIP(c.Request)
		// 环回地址放行（内网反代场景）
		if parsed := net.ParseIP(ip); parsed != nil {
			if parsed.IsLoopback() {
				c.Next()
				return
			}
			if _, ok := exact[ip]; ok {
				c.Next()
				return
			}
			for _, n := range nets {
				if n.Contains(parsed) {
					c.Next()
					return
				}
			}
		}
		log.Printf("回调来源校验失败，拒绝 [ip=%s, path=%s]", ip, c.Request.URL.Path)
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"code": 403, "message": "forbidden"})
	}
}

// clientIP 获取真实客户端 IP：优先 X-Forwarded-For 最后一跳（由 Nginx 反代透传），否则退化为 RemoteAddr
func clientIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		last := xff
		if i := strings.LastIndex(xff, ","); i >= 0 {
			last = xff[i+1:]
		}
		if ip := strings.TrimSpace(last); ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
