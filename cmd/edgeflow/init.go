package api

import (
	"context"
	"edgeflow/conf"
	"edgeflow/internal/dao/query"
	"edgeflow/internal/exchange"
	"edgeflow/internal/handler/hyperliquid"
	"edgeflow/internal/handler/instrument"
	"edgeflow/internal/handler/market"
	signal3 "edgeflow/internal/handler/signal"
	"edgeflow/internal/handler/user"
	"edgeflow/internal/router"
	"edgeflow/internal/service"
	signal2 "edgeflow/internal/service/signal"
	"edgeflow/internal/service/signal/kline"
	"edgeflow/internal/service/signal/model"
	"edgeflow/internal/service/signal/trend"
	"edgeflow/pkg/cache"
	"gorm.io/gorm"
	"time"
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
	// 信号管理
	//sm := signal.NewDefaultSignalManager(appCfg.Strategy)

	symbols := []string{"BTC/USDT", "ETH/USDT", "SOL/USDT", "DOGE/USDT", "LTC/USDT", "AAVE/USDT", "BNB/USDT", "XRP/USDT"}
	klineMgr := kline.NewKlineManager(okxEx, symbols)
	symbolMgr := model.NewSymbolManager(symbols)
	tm := trend.NewManager(okxEx, klineMgr, symbolMgr)
	signalDao := query.NewSignalDao(db)
	signalService := signal2.NewService(tm, signalDao, klineMgr, symbolMgr)

	// Trend Manager 的输入通道 (由 Kline Manager 触发)
	trendInputCh := make(chan struct{}, 1)

	// Signal Service 的输入通道 (由 Trend Manager 触发)
	signalInputCh := make(chan struct{}, 1)

	// 启动 Kline Manager (只连接到流水线的第一步)
	klineMgr.Start(trendInputCh)

	// 启动 TrendManager (计算服务)
	// TrendMgr 监听 updateTrendCh，获取最新 K线，并计算 TrendState
	tm.StartListening(context.Background(), trendInputCh, signalInputCh)
	// SignalService 同样监听 updateTrendCh，获取最新 K线和 TrendState，并生成信号
	signalService.ListenForUpdates(context.Background(), signalInputCh)

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

	hyperDao := query.NewHyperLiquidDao(db)
	defaultsCoins := []string{"BTC", "ETH", "SOL", "DOGE", "XPL", "OKB", "XRP", "LTC", "BNB", "AAVE", "AVAX", "ADA", "LINK", "TRX"}
	tickerService := service.NewOKXTickerService(defaultsCoins)
	marketService := service.NewMarketDataService(tickerService, instrumentDao)
	marketService.InitializeBaseInstruments(context.Background(), 1)

	rds := cache.GetRedisClient()
	hyperService := service.NewHyperLiquidService(hyperDao, rds, marketService)
	hyperHandler := hyperliquid.NewHandler(hyperService)

	marketHandler := market.NewMarketHandler(marketService)
	instrumentService := service.NewInstrumentService(instrumentDao, func() {
		marketService.PerformPeriodicUpdate(context.Background())
	})
	coinH := instrument.NewHandler(instrumentService)

	//tk := tokenize.NewTokenizer("./dict")
	userDao := query.NewUserDao(db)
	deviceDao := query.NewDeviceDao(db)
	deviceService := service.NewService(deviceDao)
	userService := service.NewUserService(userDao, deviceDao, deviceService)

	userHandler := user.NewUserHandler(userService, deviceService)

	signalHandler := signal3.NewSignalHandler(signalService)

	apiRouter := router.NewApiRouter(coinH, marketHandler, hyperHandler, userHandler, signalHandler)

	// 同步最新币种
	instrumentService.StartInstrumentSyncWorker(context.Background())

	// 开始广播价格
	go hyperService.StartScheduler(context.Background(), 30*time.Second)

	return apiRouter
}
