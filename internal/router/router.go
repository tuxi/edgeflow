package router

import (
	"edgeflow/internal/handler/hyperliquid"
	"edgeflow/internal/handler/instrument"
	"edgeflow/internal/handler/ticker"
	"edgeflow/internal/handler/webhook"
	"edgeflow/internal/middleware"
	"github.com/gin-gonic/gin"
)

type ApiRouter struct {
	coinHandler  *instrument.Handler
	hyperHandler *hyperliquid.Handler
	wh           *webhook.Handler
	th           *ticker.Handler
}

func NewApiRouter(ch *instrument.Handler, wh *webhook.Handler, th *ticker.Handler, hyperHandler *hyperliquid.Handler) *ApiRouter {
	return &ApiRouter{coinHandler: ch, wh: wh, th: th, hyperHandler: hyperHandler}
}

func (api *ApiRouter) Load(g *gin.Engine) {

	// auth
	base := g.Group("/api/v1")

	c := base.Group("/currencies", middleware.RequestValidationMiddleware())
	{
		// 获取币种列表
		c.GET("/list", api.coinHandler.CoinsGetList())
	}

	p := base.Group("/ticker", middleware.RequestValidationMiddleware())
	{
		p.GET("/ticker", api.th.TickerGet())   // 单币种
		p.GET("/tickers", api.th.TickersGet()) // 批量
		p.GET("/ws", api.th.ServeWS)           // 通过websocket连接获取价格

	}

	h := base.Group("/hyperliquid", middleware.RequestValidationMiddleware())
	{
		h.GET("/whales/address", api.hyperHandler.WhaleInfoGetByAddress())
		h.POST("/whales/leaderboard", api.hyperHandler.WhaleLeaderboardGet())
		h.POST("/whales/account", api.hyperHandler.WhaleAccountSummaryGet())
		h.GET("/whales/fills-orders", api.hyperHandler.WhaleUserFillOrderHistoryGet())
		h.GET("/whales/open-orders", api.hyperHandler.WhaleUserOpenOrderHistoryGet())
		h.GET("/whales/nonfunding", api.hyperHandler.WhaleUserNonFundingLedgerGet())
		h.GET("/whales/top-positions", api.hyperHandler.TopWhalePositionsGet())
		h.GET("/whales/positions-analyze", api.hyperHandler.TopWhalePositionsAnalyze())

	}

	base.POST("/webhook", middleware.RequestValidationMiddleware(), api.wh.HandlerWebhook())
}
