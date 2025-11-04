package api

import (
	"context"
	"edgeflow/conf"
	"edgeflow/internal/dao/query"
	"edgeflow/internal/handler/hyperliquid"
	"edgeflow/internal/handler/instrument"
	"edgeflow/internal/handler/market"
	signal3 "edgeflow/internal/handler/signal"
	"edgeflow/internal/handler/ticker"
	"edgeflow/internal/handler/user"
	"edgeflow/internal/router"
	"edgeflow/internal/service"
	"edgeflow/pkg/cache"
	"edgeflow/pkg/exchange"
	"edgeflow/pkg/kafka"
	"fmt"
	"gorm.io/gorm"
	"os"
)

func InitRouter(db *gorm.DB) Router {
	//tk := tokenize.NewTokenizer("./dict")
	instrumentDao := query.NewCurrenciesDao(db)

	appCfg := conf.AppConfig

	okxEx := exchange.NewOkxExchange(appCfg.Okx.ApiKey, appCfg.Okx.SecretKey, appCfg.Okx.Password)

	//d := dao.NewOrderDao(db)
	//rc := service.NewRiskService(d)

	// 仓位管理服务
	//ps := position.NewPositionService(okxEx, d)

	// k线策略
	//engine := kline.NewSignalStrategy(tm, ps, klineManger)

	//klineManger.RunScheduled(func() {
	//	tm.RunScheduled()
	//	//engine.Run(symbols)
	//})

	// hype跟单策略
	//h := hype.NewHypeTrackStrategy(ps, tm)
	//h.Run()
	//
	//go tm.RunScheduled()

	// 策略分发器：根据级别分发不同的策略
	//dispatcher := strategy.NewStrategyDispatcher()
	//dispatcher.Register("tv-level", tradingview.NewTVLevelStrategy(sm, ps, tm))

	//wh := webhook.NewHandler(dispatcher, rc, sm, ps)

	kafHost := os.Getenv("KAFKA_HOST")
	kafPort := os.Getenv("KAFKA_PORT")
	kafBroker := fmt.Sprintf("%s:%s", kafHost, kafPort)
	if kafHost == "" || kafPort == "" {
		kafBroker = conf.AppConfig.Kafka.Broker
	}

	// 初始化kafka
	kafProducer := kafka.NewKafkaProducer(kafBroker)
	kafConsumer := kafka.NewKafkaConsumer(kafBroker)

	signalDao := query.NewSignalDao(db)
	signalService := service.NewSignalProcessorService(signalDao, okxEx)
	hyperDao := query.NewHyperLiquidDao(db)
	defaultsCoins := []string{"BTC", "ETH", "SOL", "DOGE", "XPL", "OKB", "XRP", "LTC", "BNB", "AAVE", "AVAX", "ADA", "LINK", "TRX"}
	tickerService := service.NewOKXTickerService(defaultsCoins)
	marketService := service.NewMarketDataService(tickerService, instrumentDao, okxEx, signalDao, kafProducer)
	err := marketService.InitializeBaseInstruments(context.Background(), 1)
	if err != nil {
		panic(err)
	}

	rds := cache.GetRedisClient()
	hyperService := service.NewHyperLiquidService(hyperDao, rds, marketService)
	hyperHandler := hyperliquid.NewHandler(hyperService)

	okxCandleService := service.NewOKXCandleService(kafProducer)
	marketHandler := market.NewMarketHandler(marketService)
	instrumentService := service.NewInstrumentService(instrumentDao)
	coinH := instrument.NewHandler(instrumentService)

	userDao := query.NewUserDao(db)
	deviceDao := query.NewDeviceDao(db)
	deviceService := service.NewService(deviceDao)
	userService := service.NewUserService(userDao, deviceDao, deviceService)

	userHandler := user.NewUserHandler(userService, deviceService)

	signalHandler := signal3.NewSignalHandler(signalService, okxEx)

	tickerGw := ticker.NewTickerGateway(marketService, kafConsumer)
	subscriptionGw := market.NewSubscriptionGateway(okxCandleService, kafConsumer)

	apiRouter := router.NewApiRouter(coinH, marketHandler, hyperHandler, userHandler, signalHandler, tickerGw, subscriptionGw)

	return apiRouter
}
