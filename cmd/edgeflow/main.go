package main

import (
	"edgeflow/internal/config"
	"edgeflow/internal/exchange"
	"edgeflow/internal/strategy"
	"edgeflow/internal/webhook"
	"edgeflow/pkg/recorder"
	"github.com/nntaoli-project/goex/v2"
	"log"
	"net/http"
)

// 启动服务（监听webhook）

/*
测试

BODY='{"strategy":"tv-breakout-v2","symbol":"BTC/USDT","side":"buy","price":0,"quantity":0.01}'
SECRET="ab12cd34ef56abcdef1234567890abcdef1234567890abcdef1234567890"
SIGNATURE=$(echo -n $BODY | openssl dgst -sha256 -hmac $SECRET | sed 's/^.* //')

curl -X POST http://localhost:8090/webhook \
  -H "Content-Type: application/json" \
  -H "X-Signature: $SIGNATURE" \
  -d "$BODY"

*/

func main() {

	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境

	// 加载配置文件
	err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Println("WEBHOOK_SECRET = ", config.AppConfig.Webhook.Secret)

	se := exchange.NewSimulatedOrderExecutor()
	record := recorder.JSONFileRecorder{
		Path: "logs/strategy-log.json",
	}

	// 注册策略
	strategy.Register(&strategy.TVBreakoutV1{})

	strategy.Register(strategy.NewTVBreakoutV2(se, &record))

	http.HandleFunc("/webhook", webhook.HandleWebhook)

	addr := ":8090"
	log.Printf("EdgeFlow Webhook server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
