package api

import (
	"edgeflow/conf"
	"edgeflow/internal/dao"
	"edgeflow/internal/dao/query"
	"edgeflow/internal/exchange"
	"edgeflow/internal/handler/currency"
	"edgeflow/internal/handler/hyperliquid"
	"edgeflow/internal/handler/ticker"
	"edgeflow/internal/handler/webhook"
	"edgeflow/internal/position"
	"edgeflow/internal/router"
	"edgeflow/internal/service"
	"edgeflow/internal/signal"
	"edgeflow/internal/strategy"
	"edgeflow/internal/strategy/hype"
	"edgeflow/internal/strategy/tradingview"
	"edgeflow/internal/trend"
	"gorm.io/gorm"
)

func InitRouter(db *gorm.DB) Router {
	//tk := tokenize.NewTokenizer("./dict")
	cd := query.NewCurrenciesDao(db)
	ds := service.NewCoinService(cd)

	coinH := currency.NewHandler(ds)

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
	h := hype.NewHypeTrackStrategy(ps, tm)
	h.Run()

	go tm.RunScheduled()

	// 策略分发器：根据级别分发不同的策略
	dispatcher := strategy.NewStrategyDispatcher()
	dispatcher.Register("tv-level", tradingview.NewTVLevelStrategy(sm, ps, tm))

	wh := webhook.NewHandler(dispatcher, rc, sm, ps)

	tickerService, _ := service.NewOKXTickerService()
	tickerHandler := ticker.NewHandler(tickerService)

	hyperDao := query.NewHyperLiquidDao(db)

	hyperHandler := hyperliquid.NewHandler(service.NewHyperLiquidService(hyperDao))

	apiRouter := router.NewApiRouter(coinH, wh, tickerHandler, hyperHandler)

	// 开始广播价格
	go tickerHandler.BroadcastPrices()

	return apiRouter
}
