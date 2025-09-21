package service

import (
	"context"
	"edgeflow/conf"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/pkg/db"
	"log"
	"testing"
)

func TestRiskControl_OrderCreateNew(t *testing.T) {
	// 加载配置文件
	err := conf.LoadConfig("../../conf/config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// 初始化数据库
	// main.go or app bootstrap
	datasource := db.Init(db.Config{
		User:      conf.AppConfig.Username,
		Password:  conf.AppConfig.Db.Password,
		Host:      conf.AppConfig.Host,
		Port:      conf.AppConfig.Port,
		DBName:    conf.AppConfig.DbName,
		ParseTime: true,
	})
	rc := NewRiskService(dao.NewOrderDao(datasource))

	order := model.Order{
		Symbol:    "BTC/USDT",
		Side:      "buy",
		Price:     113890,
		Quantity:  1,
		OrderType: "market",
		TPPrice:   114900,
		SLPrice:   112900,
		Strategy:  "macd_ema",
		Comment:   "测试策略",
		TradeType: "swap",
		MgnMode:   "long",
	}
	err = rc.OrderCreateNew(context.Background(), order, "12002020")
	if err != nil {
		log.Fatalf("OrderCreateNew fail: %v", err)
	}

}
