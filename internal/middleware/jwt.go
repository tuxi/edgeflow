package middleware

import (
	"edgeflow/conf"
	"edgeflow/internal/consts"
	"edgeflow/pkg/jwt"
	"edgeflow/pkg/response"
	"fmt"
	"github.com/gin-gonic/gin"
	"strings"
)

// 请求头的形式为 Authorization: Bearer token
const authorizationHeader = "Authorization"

// AuthToken 鉴权，验证用户token是否有效
func AuthToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenStr, err := getJwtFromHeader(c)
		if err != nil {
			response.RequireAuthErr(c, err)
			c.Abort()
			return
		}
		if jwt.IsInBlackList(c, tokenStr) {
			response.RequireAuthErr(c, err)
			c.Abort()
			return
		}
		// 验证token是否正确

		claims, err := jwt.ParseToken(tokenStr, conf.AppConfig.Jwt.Secret)
		if err != nil {
			response.RequireAuthErr(c, err)
			c.Abort()
			return
		}

		c.Set(consts.UserID, claims.UserId)
		c.Set(consts.JWTTokenCtx, tokenStr)
		c.Next()
	}
}

func getJwtFromHeader(c *gin.Context) (string, error) {
	aHeader := c.Request.Header.Get(authorizationHeader)
	if len(aHeader) == 0 {
		return "", fmt.Errorf("token is empty")
	}
	strs := strings.SplitN(aHeader, " ", 2)
	if len(strs) != 2 || strs[0] != "Bearer" {
		return "", fmt.Errorf("token 不符合规则")
	}
	return strs[1], nil
}
