package cache

import (
	"context"
	"edgeflow/conf"

	"github.com/redis/go-redis/v9"
)

var redisClient *redis.Client

// InitRedis 初始化redisClient
func InitRedis(redisCfg conf.RedisConfig) {
	redisClient = redis.NewClient(&redis.Options{
		DB:           redisCfg.Db,
		Addr:         redisCfg.Addr,
		Password:     redisCfg.Password,
		PoolSize:     redisCfg.PoolSize,
		MinIdleConns: redisCfg.MinIdleConns,
	})
	_, err := redisClient.Ping(context.TODO()).Result()
	if err != nil {
		panic(err)
	}
}

func GetRedisClient() *redis.Client {
	if nil == redisClient {
		panic("Please initialize the Redis client first!")
	}
	return redisClient
}

// 关闭redis client
func CloseRedis() {
	if nil != redisClient {
		_ = redisClient.Close()
	}
}
