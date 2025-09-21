package cache

import (
	"context"
	"edgeflow/conf"
	"github.com/go-redis/redis/v8"
	"time"
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
		IdleTimeout:  time.Duration(redisCfg.IdleTimeout) * time.Second,
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
