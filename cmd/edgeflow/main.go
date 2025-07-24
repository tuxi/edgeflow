package main

import (
	"edgeflow/internal/config"
	"edgeflow/internal/strategy"
	"edgeflow/internal/webhook"
	"log"
	"net/http"
)

// 启动服务（监听webhook）

func main() {

	// 加载配置文件
	err := config.LoadConfig("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Println("WEBHOOK_SECRET = ", config.AppConfig.Webhook.Secret)

	// 注册策略
	strategy.Register(&strategy.TVBreakoutV1{})

	http.HandleFunc("/webhook", webhook.HandleWebhook)

	addr := ":8090"
	log.Printf("EdgeFlow Webhook server listening on %s\n", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
