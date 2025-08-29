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
