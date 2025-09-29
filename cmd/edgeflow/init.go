package api

import (
	"context"
	"edgeflow/conf"
	"edgeflow/internal/dao"
	"edgeflow/internal/dao/query"
	"edgeflow/internal/exchange"
	"edgeflow/internal/handler/hyperliquid"
	"edgeflow/internal/handler/instrument"
	"edgeflow/internal/handler/market"
	"edgeflow/internal/handler/webhook"
	"edgeflow/internal/position"
	"edgeflow/internal/router"
	"edgeflow/internal/service"
	"edgeflow/internal/signal"
	"edgeflow/internal/strategy"
	"edgeflow/internal/strategy/tradingview"
	"edgeflow/internal/trend"
	"edgeflow/pkg/cache"
	"gorm.io/gorm"
	"time"
)

func InitRouter(db *gorm.DB) Router {
	//tk := tokenize.NewTokenizer("./dict")
	instrumentDao := query.NewCurrenciesDao(db)

	appCfg := conf.AppConfig

	okxEx := exchange.NewOkxExchange(appCfg.Okx.ApiKey, appCfg.Okx.SecretKey, appCfg.Okx.Password)

	d := dao.NewOrderDao(db)
	rc := service.NewRiskService(d)

	// 仓位管理服务
	ps := position.NewPositionService(okxEx, d)
	// 信号管理
	sm := signal.NewDefaultSignalManager(appCfg.Strategy)

	symbols := []string{"BTC/USDT", "ETH/USDT", "SOL/USDT", "DOGE/USDT", "HYPE/USDT", "LTC/USDT"}
	klineManger := trend.NewKlineManager(okxEx, symbols)
	tm := trend.NewManager(okxEx, symbols, klineManger)

	// k线策略
	//engine := kline.NewSignalStrategy(tm, ps, klineManger)

	klineManger.RunScheduled(func() {
		tm.RunScheduled()
		//engine.Run(symbols)
	})

	// hype跟单策略
	//h := hype.NewHypeTrackStrategy(ps, tm)
	//h.Run()
	//
	//go tm.RunScheduled()

	// 策略分发器：根据级别分发不同的策略
	dispatcher := strategy.NewStrategyDispatcher()
	dispatcher.Register("tv-level", tradingview.NewTVLevelStrategy(sm, ps, tm))

	wh := webhook.NewHandler(dispatcher, rc, sm, ps)

	hyperDao := query.NewHyperLiquidDao(db)

	rds := cache.GetRedisClient()
	hyperService := service.NewHyperLiquidService(hyperDao, rds)
	hyperHandler := hyperliquid.NewHandler(hyperService)

	defaultsCoins := []string{"BTC", "ETH", "SOL", "DOGE", "XPL", "OKB", "XRP", "LTC", "BNB", "AAVE", "AVAX", "ADA", "LINK", "TRX"}
	tickerService := service.NewOKXTickerService(defaultsCoins)
	marketService := service.NewMarketDataService(tickerService, instrumentDao)
	marketService.InitializeBaseInstruments(context.Background(), 1)
	marketHandler := market.NewMarketHandler(marketService)
	instrumentService := service.NewInstrumentService(instrumentDao, func() {
		marketService.PerformPeriodicUpdate(context.Background())
	})
	coinH := instrument.NewHandler(instrumentService)

	apiRouter := router.NewApiRouter(coinH, wh, marketHandler, hyperHandler)

	// 同步最新币种
	instrumentService.StartInstrumentSyncWorker(context.Background())

	// 开始广播价格
	go hyperService.StartScheduler(context.Background(), 30*time.Second)

	return apiRouter
}
