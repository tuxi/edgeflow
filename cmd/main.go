package main

import (
	"edgeflow/internal/config"
	"edgeflow/internal/dao"
	"edgeflow/internal/exchange"
	"edgeflow/internal/risk"
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

BODY='{"strategy":"tv-breakout-v3","symbol":"BTC/USDT","side":"buy","price":113990,"quantity":0.01,"order_type":"market","trade_type":"swap","tp_pct":0.5,"sl_pct":0.3,"leverage":20,"score": 4,"level": 1,"timestamp": "2025-08-09T21:54:30+08:00"}'
SECRET="ab12cd34ef56abcdef1234567890abcdef1234567890abcdef1234567890"
SIGNATURE=$(echo -n $BODY | openssl dgst -sha256 -hmac $SECRET | sed 's/^.* //')

curl -X POST http://localhost:8090/webhook \
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
	rc := risk.NewRiskControl(dao.NewOrderDao(datasource))

	log.Println("WEBHOOK_SECRET = ", config.AppConfig.Webhook.Secret)

	okxCf := config.AppConfig.Okx
	okxEx := exchange.NewOkxExchange(okxCf.ApiKey, okxCf.SecretKey, okxCf.Password)

	strategy.Register(strategy.NewTVBreakoutV2(okxEx, rc))

	http.HandleFunc("/webhook", webhook.HandleWebhook)

	addr := ":12180"
	log.Printf("EdgeFlow Webhook server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
