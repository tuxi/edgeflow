package main

import (
	"edgeflow/cmd/edgeflow"
	"edgeflow/conf"
	"edgeflow/internal/middleware"
	"edgeflow/pkg/cache"
	"edgeflow/pkg/db"
	"edgeflow/pkg/logger"
	"fmt"
	"github.com/nntaoli-project/goex/v2"
	"log"
	"os"
)

// 启动服务（监听webhook）

/*
测试

BODY='{"strategy":"tv-breakout-v3","symbol":"BTC/USDT","side":"buy","price":113990,"quantity":0.01,"order_type":"market","trade_type":"swap","tp_pct":0.5,"sl_pct":0.3,"leverage":20,"score": 4,"level": 1,"timestamp": "2025-08-10T21:54:30+08:00"}'
SECRET="ab12cd34ef56abcdef1234567890abcdef1234567890abcdef1234567890"
SIGNATURE=$(echo -n $BODY | openssl dgst -sha256 -hmac $SECRET | sed 's/^.* //')

curl -X POST http://localhost:12180/webhook \
  -H "Content-Type: application/json" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"

BODY='{"comment":"空头进场信号","symbol":"ETH/USDT","timestamp":"2025-08-15T23:50:04Z","side":"sell","type":"entry","level":2,"trade_type":"swap","tp_pct":0.35,"sl_pct":0.3,"strategy":"macd-ema-v6","price":4324.7,"order_type":"market"}'
SECRET="ab12cd34ef56abcdef1234567890abcdef1234567890abcdef1234567890"
SIGNATURE=$(echo -n $BODY | openssl dgst -sha256 -hmac $SECRET | sed 's/^.* //')

curl -X POST http://localhost:12180/webhook \
  -H "Content-Type: application/json" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"

BODY='{"comment":"进场信号","symbol":"BTC/USDT","timestamp":"2025-08-31T13:23:04Z","side":"buy","type":"entry","level":1,"trade_type":"swap","tp_pct":0.35,"sl_pct":0.3,"strategy":"tv-level","price":4324.7,"order_type":"market"}'
SECRET="ab12cd34ef56abcdef1234567890abcdef1234567890abcdef1234567890"
SIGNATURE=$(echo -n $BODY | openssl dgst -sha256 -hmac $SECRET | sed 's/^.* //')

curl -X POST http://localhost:12180/webhook \
  -H "Content-Type: application/json" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"
*/

func main() {

	// 加载配置文件
	err := conf.LoadConfig("conf/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	appCfg := conf.AppConfig
	logger.InitLogger(&appCfg.Log, appCfg.AppName)
	if conf.AppConfig.Simulated {
		// 设置为模拟环境
		goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	if dbUser == "" || dbPass == "" || dbHost == "" {
		dbUser = conf.AppConfig.Username
		dbPass = conf.AppConfig.Db.Password
		dbHost = conf.AppConfig.Host
		dbPort = conf.AppConfig.Port
		dbName = conf.AppConfig.DbName
	}

	// 初始化数据库
	// main.go or app bootstrap
	datasource := db.Init(db.Config{
		User:      dbUser,
		Password:  dbPass,
		Host:      dbHost,
		Port:      dbPort,
		DBName:    dbName,
		ParseTime: true,
	})

	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")
	redisPassword := os.Getenv("REDIS_PASSWORD")
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort)
	if redisHost == "" || redisPort == "" {
		redisAddr = conf.AppConfig.Redis.Addr
	}
	if redisPassword != "" {
		appCfg.Redis.Password = redisPassword
	}
	appCfg.Redis.Addr = redisAddr

	// 初始化redis缓存
	cache.InitRedis(appCfg.Redis)

	// 创建并启动服务
	srv := api.NewServer(&appCfg)
	srv.RegisterOnShutdown(func() {
		if datasource != nil {
			// 关闭主库链接
			m, err := datasource.DB()
			if err != nil {
				_ = m.Close()
			}
		}

		cache.CloseRedis()

	})
	srvRouter := api.InitRouter(datasource)

	srv.Run(middleware.NewMiddleware(), srvRouter)
}
