package trend

import (
	"edgeflow/internal/config"
	"edgeflow/internal/exchange"
	model2 "github.com/nntaoli-project/goex/v2/model"
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

	symbols := []string{"BTC/USDT", "ETH/USDT"}
	tm := NewTrendManager(okx, symbols, model2.Kline_4h)
	tm.StartUpdater()

	// 查询某币种趋势
	for {
		state, ok := tm.Get("BTC/USDT")
		if ok {
			log.Printf("BTC Trend: %v, MA200: %.2f EMA50: %.2f ADX14: %.2f lastPrice: %.2f", state.Direction, state.MA200, state.EMA50, state.ADX, state.LastPrice)
		}

		state1, ok := tm.Get("ETH/USDT")
		if ok {
			log.Printf("ETH Trend: %v, MA200: %.2f EMA50: %.2f ADX14: %.2f lastPrice: %.2f", state1.Direction, state1.MA200, state1.EMA50, state1.ADX, state1.LastPrice)
		}

		time.Sleep(time.Minute * 1)
	}

}
