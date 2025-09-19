package strategy

import (
	"edgeflow/internal/strategy/hype"
	"testing"
)

func TestStrategyEngine_Run(t *testing.T) {
	// 加载配置文件
	//err := config.LoadConfig("../../conf/config.yaml")
	//if err != nil {
	//	log.Fatalf("Failed to load config: %v", err)
	//}
	//
	//if config.AppConfig.Simulated {
	//	// 设置为模拟环境
	//	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1")
	//}
	//
	//dbUser := os.Getenv("DB_USER")
	//dbPass := os.Getenv("DB_PASSWORD")
	//dbHost := os.Getenv("DB_HOST")
	//dbPort := os.Getenv("DB_PORT")
	//dbName := os.Getenv("DB_NAME")
	//if dbUser == "" || dbPass == "" || dbHost == "" {
	//	dbUser = config.AppConfig.Username
	//	dbPass = config.AppConfig.Db.Password
	//	dbHost = config.AppConfig.Host
	//	dbPort = config.AppConfig.Port
	//	dbName = config.AppConfig.DbName
	//}
	//
	//// 初始化数据库
	//// main.go or app bootstrap
	//datasource := db.Init(db.Config{
	//	User:      dbUser,
	//	Password:  dbPass,
	//	Host:      dbHost,
	//	Port:      dbPort,
	//	DBName:    dbName,
	//	ParseTime: true,
	//})
	//d := dao.NewOrderDao(datasource)
	//
	//log.Println("WEBHOOK_SECRET = ", config.AppConfig.Webhook.Secret)
	//
	//appCfg := config.AppConfig
	//okxEx := exchange.NewOkxExchange(appCfg.Okx.ApiKey, appCfg.Okx.SecretKey, appCfg.Okx.Password)
	//
	//// 仓位管理服务
	//ps := position.NewPositionService(okxEx, d)
	//
	//tm := trend.NewManager(okxEx, []string{"BTC/USDT", "ETH/USDT", "SOL/USDT"})
	//tm.StartUpdater()
	//
	//// 注册指标
	//sg := trend.NewSignalGenerator()
	//
	//engine := NewSignalStrategy(tm, sg, ps, true)
	//engine.Run(time.Minute*10, []string{"BTC/USDT", "ETH/USDT", "SOL/USDT"})
}

func TestHypeTrackStrategy_Run(t *testing.T) {
	hyper := hype.NewHypeTrackStrategy()
	hyper.Run()
}
