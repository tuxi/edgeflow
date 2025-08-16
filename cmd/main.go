package main

import (
	"edgeflow/internal/config"
	"edgeflow/internal/dao"
	"edgeflow/internal/exchange"
	"edgeflow/internal/service"
	"edgeflow/internal/strategy"
	"edgeflow/internal/webhook"
	"edgeflow/pkg/db"
	"github.com/nntaoli-project/goex/v2"
	"log"
	"net/http"
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
*/

func main() {

	// 加载配置文件
	err := config.LoadConfig("conf/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if config.AppConfig.Simulated {
		// 设置为模拟环境
		goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	if dbUser == "" || dbPass == "" || dbHost == "" {
		dbUser = config.AppConfig.Username
		dbPass = config.AppConfig.Db.Password
		dbHost = config.AppConfig.Host
		dbPort = config.AppConfig.Port
		dbName = config.AppConfig.DbName
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
	d := dao.NewOrderDao(datasource)
	rc := service.NewRiskService(d)

	log.Println("WEBHOOK_SECRET = ", config.AppConfig.Webhook.Secret)

	okxCf := config.AppConfig.Okx
	okxEx := exchange.NewOkxExchange(okxCf.ApiKey, okxCf.SecretKey, okxCf.Password)

	// 仓位管理服务
	ps := service.NewPositionService(okxEx, d)
	// 信号管理
	sm := service.NewDefaultSignalManager()

	// 策略分发器：根据级别分发不同的策略
	dispatcher := strategy.NewStrategyDispatcher()
	dispatcher.Register(1, strategy.NewTVTrendH(sm, ps))
	dispatcher.Register(2, strategy.NewTVScalp15M(sm, ps))
	dispatcher.Register(3, strategy.NewTVScalp15M(sm, ps))

	hander := webhook.NewWebhookHandler(dispatcher, rc)

	http.HandleFunc("/webhook", hander.HandleWebhook)

	addr := ":12180"
	log.Printf("EdgeFlow Webhook server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
