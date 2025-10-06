package active

import (
	"context"
	"edgeflow/pkg/cache"
	"edgeflow/pkg/logger"
	"edgeflow/utils/security"
	"edgeflow/utils/uuid"
	"github.com/go-redis/redis/v8"
	"strconv"
	"time"
)

func getActiveCodeKey(code string) string {
	return "User_Active_Code_list:" + security.Md5(code)
}

func activeCodeSave(ctx context.Context, code string, userId int64) (err error) {
	timer := 172800 * time.Second
	rc := cache.GetRedisClient()
	err = rc.SetNX(ctx, getActiveCodeKey(code), userId, timer).Err()
	return err
}

func ActiveCodeCompare(ctx context.Context, code string, userId int64) bool {
	rc := cache.GetRedisClient()
	idstr, err := rc.Get(ctx, getActiveCodeKey(code)).Result()
	if err != nil {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		logger.Debugf("用户：%d,激活代码%s不存在", userId, code)
		return false
	}
	id, err := strconv.ParseInt(idstr, 10, 64)
	if id != userId || err != nil {
		return false
	}
	rc.Del(ctx, getActiveCodeKey(code))
	return true
}

func ActiveCodeGen(ctx context.Context, userId int64) (string, error) {
	uid := uuid.GenUUID16()
	code := strconv.FormatInt(userId, 10) + uid
	err := activeCodeSave(ctx, code, userId)
	return code, err
}
