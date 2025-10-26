package hype

import (
	"edgeflow/conf"
	"edgeflow/internal/dao"
	"edgeflow/internal/position"
	"edgeflow/internal/trend"
	"edgeflow/pkg/db"
	"edgeflow/pkg/exchange"
	"edgeflow/pkg/hype/types"
	"encoding/json"
	"github.com/nntaoli-project/goex/v2"
	"log"
	"os"
	"testing"
	"time"
)

func loadOkxConf() (*conf.Okx, error) {
	// 加载配置文件
	err := conf.LoadConfig("../../../conf/config.yaml")
	if err != nil {
		return nil, err
	}

	return &conf.AppConfig.Okx, nil
}

func TestTradeOrders(t *testing.T) {
	// 加载配置文件
	// 加载配置文件
	okxConf, err := loadOkxConf()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if conf.AppConfig.Simulated {
		// 设置为模拟环境
		goex.DefaultHttpCli.SetHeaders("x-simulated-trading", "1")
	}

	dbUser := os.Getenv("DB_USER")
	dbPass := os.Getenv("DB_PASSWORD")
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	if dbUser == "" || dbPass == "" || dbHost == "" {
		dbUser = conf.AppConfig.Username
		dbPass = conf.AppConfig.Db.Password
		dbHost = conf.AppConfig.Host
		dbPort = conf.AppConfig.Port
		dbName = conf.AppConfig.DbName
	}

	// 初始化数据库
	// main.go or app bootstrap
	datasource := db.Init(db.Config{
		User:      dbUser,
		Password:  dbPass,
		Host:      dbHost,
		Port:      dbPort,
		DBName:    dbName,
		ParseTime: true,
	})
	d := dao.NewOrderDao(datasource)

	log.Println("WEBHOOK_SECRET = ", conf.AppConfig.Webhook.Secret)

	appCfg := okxConf
	okxEx := exchange.NewOkxExchange(appCfg.ApiKey, appCfg.SecretKey, appCfg.Password)

	symbols := []string{"BTC/USDT", "ETH/USDT", "SOL/USDT", "DOGE/USDT", "HYPE/USDT", "LTC/USDT"}
	klineManger := trend.NewKlineManager(okxEx, symbols)
	tm := trend.NewManager(okxEx, symbols, klineManger)

	// 仓位管理服务
	ps := position.NewPositionService(okxEx, d)
	h := NewHypeTrackStrategy(ps, tm)
	h.Run()

	//raw := []byte(`{"channel":"orderUpdates","data":[{"order":{"coin":"BTC","side":"B","limitPx":"126192.0","sz":"5.0","oid":166873653063,"timestamp":1758238143369,"origSz":"5.0"},"status":"open","statusTimestamp":1758238143369},{"order":{"coin":"BTC","side":"B","limitPx":"126192.0","sz":"0.0","oid":166873653063,"timestamp":1758238143369,"origSz":"5.0"},"status":"filled","statusTimestamp":1758238143369}]}`)
	raw := []byte(`{"channel":"orderUpdates","data":[{"order":{"coin":"BTC","side":"B","limitPx":"126192.0","sz":"5.0","oid":166873653063,"timestamp":1758238143369,"origSz":"5.0"},"status":"open","statusTimestamp":1758238143369},{"order":{"coin":"HYPE","side":"B","limitPx":"126192.0","sz":"0.0","oid":166873653063,"timestamp":1758238143369,"origSz":"5.0"},"status":"filled","statusTimestamp":1758238143369}]}`)

	var res types.OrderUpdatesMessage

	err = json.Unmarshal(raw, &res)
	if err != nil {
		log.Printf("解析json错误： %v", err)
		return
	}

	// 延迟一会，等待数据初始化完成
	time.Sleep(time.Second * 10)
	h.ReceiveOrders(res.Data)
}
