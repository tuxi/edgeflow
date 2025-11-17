package main

import (
	"edgeflow/cmd/edgeflow"
	"edgeflow/conf"
	"edgeflow/internal/middleware"
	"edgeflow/pkg/cache"
	"edgeflow/pkg/db"
	"edgeflow/pkg/logger"
	"fmt"
	"log"
	"os"

	"github.com/nntaoli-project/goex/v2"
)

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
