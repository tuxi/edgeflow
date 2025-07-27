package exchange

import (
	"context"
	"edgeflow/internal/config"
	"edgeflow/internal/model"
	"fmt"
	"log"
	"testing"
)

func loadOkxConf() (*config.Okx, error) {
	// 加载配置文件
	err := config.LoadConfig("../../config.yaml")
	if err != nil {
		return nil, err
	}

	return &config.AppConfig.Okx, nil
}

func TestOkxExchange_GetLastPrice(t *testing.T) {

	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	okxEx, err := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)
	if err != nil {
		panic(err)
	}

	price, err := okxEx.GetLastPrice("BTC/USDT")
	fmt.Println("最新价格:", price, err)

}

func TestOkxExchange_PlaceOrder(t *testing.T) {

	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	okxEx, err := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)
	if err != nil {
		panic(err)
	}

	order := model.Order{
		Symbol:    "BTC/USDT",
		Side:      model.Buy,
		Price:     117980,
		Quantity:  0.0001,
		OrderType: model.Market,
		TPPrice:   0,
		SLPrice:   0,
		Strategy:  "Stragety1",
		Comment:   "测试",
	}
	resp, err := okxEx.PlaceOrder(context.Background(), order)

	if resp.OrderId == "" {
		t.Errorf("Expected non-empty order ID")
	} else {
		t.Logf("Order ID: %s", resp.OrderId)
	}
}
