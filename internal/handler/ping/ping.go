package ping

import (
	"github.com/gin-gonic/gin"
	"net"
	"net/http"
)

func Ping() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		ctx.String(http.StatusOK, "\r\nSuccess")
	}
}

// 检测请求的ip是否是本地ip
func isLocalIP(host string) bool {
	ip, _, err := net.SplitHostPort(host)
	if err != nil {
		return false
	}
	allowIps := []string{"localhost", "127.0.0.1"}
	for _, item := range allowIps {
		if ip == item {
			return true
		}
	}
	return false
}
