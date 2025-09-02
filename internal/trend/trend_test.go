package trend

import (
	"edgeflow/internal/config"
	"edgeflow/internal/exchange"
	"log"
	"testing"
	"time"
)

func loadOkxConf() (*config.Okx, error) {
	// 加载配置文件
	err := config.LoadConfig("../../conf/config.yaml")
	if err != nil {
		return nil, err
	}

	return &config.AppConfig.Okx, nil
}

func TestTrend(t *testing.T) {

	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	okx := exchange.NewOkxExchange(okxConf.ApiKey, okxConf.SecretKey, okxConf.Password)

	symbols := []string{"BTC/USDT", "ETH/USDT", "SOL/USDT"}
	tm := NewManager(okx, symbols)
	tm.StartUpdater()

	// 查询某币种趋势
	for {
		state, ok := tm.Get("BTC/USDT")
		if ok {
			log.Println(state.Description)
		}

		state1, ok := tm.Get("ETH/USDT")
		if ok {
			log.Println(state1.Description)
		}
		state2, ok := tm.Get("SOL/USDT")
		if ok {
			log.Println(state2.Description)
		}

		time.Sleep(time.Minute * 1)
	}

}

func TestIndicator(t *testing.T) {
	// 模拟 TrendManager 拉到的K线
	//klines := []model.Kline{
	//	{time.Now().Add(-5 * time.Minute), 10800, 10820, 10790, 10810, 100},
	//	{time.Now().Add(-4 * time.Minute), 10810, 10830, 10795, 10820, 120},
	//	{time.Now().Add(-3 * time.Minute), 10820, 10840, 10800, 10835, 130},
	//	{time.Now().Add(-2 * time.Minute), 10835, 10850, 10810, 10825, 150},
	//	{time.Now().Add(-1 * time.Minute), 10825, 10860, 10800, 10855, 180},
	//}
	//
	//// 注册指标
	//sg := &SignalGenerator{
	//	Indicators: []Indicator{
	//		&EMAIndicator{FastPeriod: 5, SlowPeriod: 10},
	//		&MACDIndicator{FastPeriod: 12, SlowPeriod: 26, SignalPeriod: 9},
	//		&RSIIndicator{Period: 14, Buy: 30, Sell: 70},
	//	},
	//}

	// 生成信号
	//signal := sg.Generate(klines, "BTC-USDT")
	//fmt.Printf("Final TradeSignal: %+v\n", signal)
}
