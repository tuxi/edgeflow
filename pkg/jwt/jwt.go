package jwt

import (
	"context"
	"edgeflow/conf"
	"edgeflow/pkg/cache"
	"edgeflow/pkg/logger"
	"edgeflow/utils/security"
	"github.com/go-redis/redis/v8"
	"github.com/golang-jwt/jwt/v4"
	"strconv"
	"strings"
	"time"
)

type CustomClaims struct {
	UserId int64  `json:"user_id"`
	Sub    string `json:"sub"` // 鉴权的主题，目前有user 和 anonymous两种
	RoleId int    `json:"role_id"`
	jwt.RegisteredClaims
}

// 是否为匿名用户
func (claims *CustomClaims) IsAnonymousUser() bool {
	return strings.HasPrefix(claims.Sub, "anonymous")
}

// 是否为管理员
func (claims *CustomClaims) IsAdministrator() bool {
	arr := strings.Split(claims.Sub, "_")
	if len(arr) == 2 && arr[1] == "administrator" {
		return true
	}
	return false
}

func BuildClaims(exp time.Time, uid int64, rid int, isAnonymous, isAdministrator bool) *CustomClaims {
	var sub = "user"
	if isAnonymous {
		sub = "anonymous"
	}
	if isAdministrator {
		sub = sub + "_administrator"
	}
	return &CustomClaims{
		UserId: uid,
		Sub:    sub,
		RoleId: rid,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    conf.AppConfig.AppName,
		},
	}
}

func GenToken(c *CustomClaims, secretKey string) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	ss, err := token.SignedString([]byte(secretKey))
	return ss, err
}

// 解析jwt token
func ParseToken(jwtStr, secretKey string) (*CustomClaims, error) {
	token, err := jwt.ParseWithClaims(jwtStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*CustomClaims); ok && token.Valid {
		return claims, err
	} else {
		return nil, err
	}
}

func getBlackListKey(token string) string {
	return "jwt_black_list:" + security.Md5(token)
}

func JoinBlackList(ctx context.Context, tokenStr string, secretKey string) (err error) {
	token, err := jwt.ParseWithClaims(tokenStr, &CustomClaims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(secretKey), nil
	})
	if err != nil {
		return err
	}
	nowUnix := time.Now().Unix()
	timer := time.Duration(token.Claims.(*CustomClaims).ExpiresAt.Unix()-nowUnix) * time.Second
	rc := cache.GetRedisClient()
	err = rc.SetNX(ctx, getBlackListKey(token.Raw), nowUnix, timer).Err()
	return
}

func IsInBlackList(ctx context.Context, token string) bool {
	rc := cache.GetRedisClient()
	joinUnixStr, err := rc.Get(ctx, getBlackListKey(token)).Result()
	if err != nil {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		return false
	}
	joinUnix, err := strconv.ParseInt(joinUnixStr, 10, 64)
	if time.Now().Unix()-joinUnix < conf.AppConfig.Jwt.JwtBlacklistGracePeriod {
		return false
	}
	return true
}
