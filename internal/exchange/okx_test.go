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

	okxEx := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)

	price, err := okxEx.GetLastPrice("BTC/USDT", model.OrderTradeSpot)
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

	okxEx := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)
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

	resp, err := okxEx.PlaceOrder(context.Background(), &order)

	if resp.OrderId == "" {
		t.Errorf("Expected non-empty order ID")
	} else {
		t.Logf("Order ID: %s", resp.OrderId)

		// 获取订单状态
		status, err := okxEx.GetOrderStatus(resp.OrderId, order.Symbol, model.OrderTradeSpot)
		if err != nil {
			t.Logf("Order status: %v", status.Status)
		}

		// 如果没有完成，测试撤单
		if status.Status == "pending" {
			err := okxEx.CancelOrder(resp.OrderId, order.Symbol, model.OrderTradeSpot)
			if err == nil {
				t.Log("取消订单完成")
			}
		}
	}
}

// 测试市价下单现货，并且带有止盈止损
func TestOkxExchange_PlaceOrderSpot(t *testing.T) {
	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境
	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	okxEx := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)

	order := model.Order{
		Symbol:    "SOL/USDT",
		Side:      model.Buy,
		Price:     192.1,
		Quantity:  1, // 限价Quantity 单位是币本身 == 1SOL
		OrderType: model.Limit,
		TPPrice:   192.2,
		SLPrice:   190,
		Strategy:  "Stragety1",
		Comment:   "测试限价购买",
		TradeType: model.OrderTradeSpot,
	}

	// 获取市价
	lastPrice, err := okxEx.GetLastPrice(order.Symbol, order.TradeType)
	if err != nil {
		order.SLPrice = 0
		fmt.Printf("获取最新价格失败：%v", err)
	} else {
		// 订单的价格太偏离当前市场价，或止盈止损设置不符合规则，会触发okx的系统封控，报错"sCode": "51137",
		order.Price = lastPrice
		// 将止损价格设置低于最新价格一些，不然无法下单
		if order.SLPrice > 0 {
			order.SLPrice = lastPrice - 5
		}
	}

	resp, err := okxEx.PlaceOrder(context.Background(), &order)

	if resp.OrderId == "" {
		t.Errorf("Expected non-empty order ID")
	} else {
		t.Logf("Order ID: %s", resp.OrderId)
	}
}

// 测试市价下单永续合约，并且带有止盈止损
func TestOkxExchange_PlaceOrderSwap(t *testing.T) {
	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境
	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	okxEx := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)

	order := model.Order{
		Symbol:    "SOL/USDT",
		Side:      model.Buy,
		Price:     192.1,
		Quantity:  1, // 限价Quantity 单位是币本身 == 1SOL
		OrderType: model.Limit,
		TPPrice:   192.2,
		SLPrice:   190,
		Strategy:  "Stragety1",
		Comment:   "测试限价购买",
		TradeType: model.OrderTradeSwap,
	}

	// 获取市价
	lastPrice, err := okxEx.GetLastPrice(order.Symbol, order.TradeType)
	if err != nil {
		order.SLPrice = 0
		fmt.Printf("获取最新价格失败：%v", err)
	} else {
		// 订单的价格太偏离当前市场价，或止盈止损设置不符合规则，会触发okx的系统封控，报错"sCode": "51137",
		order.Price = lastPrice
		// 将止损价格设置低于最新价格一些，不然无法下单
		if order.SLPrice > 0 {
			order.SLPrice = lastPrice - 5
		}
	}

	resp, err := okxEx.PlaceOrder(context.Background(), &order)

	if resp.OrderId == "" {
		t.Errorf("Expected non-empty order ID")
	} else {
		t.Logf("Order ID: %s", resp.OrderId)
	}
}

func TestOkxChange_SetLeverage(t *testing.T) {
	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境
	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	okxEx := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)

	err = okxEx.SetLeverage("SOL/USDT", 50, model.OrderMgnModeIsolated, model.OrderPosSideLong)
	if err != nil {
		t.Errorf("SetLeverage error: %v", err)
	}
}

func TestOkxExchange_GetPosition(t *testing.T) {
	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境
	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	okxEx := NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)

	ps, err := okxEx.GetPosition("BTC/USDT")
	if err != nil {
		t.Errorf("GetPosition error: %v", err)
	} else {
		fmt.Printf("持有仓位：%v", ps)

		// 测试平仓
		for _, position := range ps {
			err := okxEx.ClosePosition(position.Symbol, string(position.Side), position.Amount, position.MgnMode)
			if err != nil {
				fmt.Printf("平仓失败：%v", err)
			} else {
				fmt.Println("平仓成功")
			}
		}
	}

}
