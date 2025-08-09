package account

import (
	"context"
	"edgeflow/internal/config"
	"github.com/nntaoli-project/goex/v2"
	"github.com/nntaoli-project/goex/v2/options"
	"log"
	"testing"
)

func TestAccountService_GetBalance(t *testing.T) {
	goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1") // 设置为模拟环境
	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	opts := []options.ApiOption{
		options.WithApiKey(okxConf.ApiKey),
		options.WithApiSecretKey(okxConf.SecretKey),
		options.WithPassphrase(okxConf.Password),
	}

	prv := goex.OKx.Spot.NewPrvApi(opts...)
	as := NewAccountService(prv)
	avail, err := as.GetAccount(context.Background(), "USDT")
	if err != nil {
		log.Fatalf("查询余额失败: %v", err)
	}

	log.Printf("USDT 可用余额: %.6f", avail.Available)
}

func loadOkxConf() (*config.Okx, error) {
	// 加载配置文件
	err := config.LoadConfig("../../conf/config.yaml")
	if err != nil {
		return nil, err
	}

	return &config.AppConfig.Okx, nil
}
