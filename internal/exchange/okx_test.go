package exchange

import (
	"context"
	"edgeflow/internal/config"
	"edgeflow/internal/model"
	"fmt"
	"github.com/nntaoli-project/goex/v2"
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

// 测试限价下单
func TestOkxExchange_PlaceOrder(t *testing.T) {
	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境
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
		Symbol:    "SOL/USDT",
		Side:      model.Buy,
		Price:     100,
		Quantity:  1, // 市价Quantity 单位是USDT == 1USDT
		OrderType: model.Market,
		TPPrice:   0,
		SLPrice:   0,
		Strategy:  "Stragety1",
		Comment:   "测试市价购买",
	}

	resp, err := okxEx.PlaceOrder(context.Background(), order)

	if resp.OrderId == "" {
		t.Errorf("Expected non-empty order ID")
	} else {
		t.Logf("Order ID: %s", resp.OrderId)

		// 获取订单状态
		status, err := okxEx.GetOrderStatus(resp.OrderId, order.Symbol)
		if err != nil {
			t.Logf("Order status: %v", status.Status)
		}

		// 如果没有完成，测试撤单
		if status.Status == "pending" {
			err := okxEx.CancelOrder(resp.OrderId, order.Symbol)
			if err == nil {
				t.Log("取消订单完成")
			}
		}
	}
}

// 测试市价下单，并且带有止盈止损
func TestOkxExchange_PlaceOrder1(t *testing.T) {
	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境
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
		Symbol:    "SOL/USDT",
		Side:      model.Buy,
		Price:     192.1,
		Quantity:  1, // 限价Quantity 单位是币本身 == 1SOL
		OrderType: model.Limit,
		TPPrice:   192.2,
		SLPrice:   192,
		Strategy:  "Stragety1",
		Comment:   "测试限价购买",
	}

	resp, err := okxEx.PlaceOrder(context.Background(), order)

	if resp.OrderId == "" {
		t.Errorf("Expected non-empty order ID")
	} else {
		t.Logf("Order ID: %s", resp.OrderId)
	}
}
