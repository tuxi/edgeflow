package router

import (
	"edgeflow/internal/handler/currency"
	"edgeflow/internal/handler/hyperliquid"
	"edgeflow/internal/handler/ticker"
	"edgeflow/internal/handler/webhook"
	"edgeflow/internal/middleware"
	"github.com/gin-gonic/gin"
)

type ApiRouter struct {
	coinHandler  *currency.Handler
	hyperHandler *hyperliquid.Handler
	wh           *webhook.Handler
	th           *ticker.Handler
}

func NewApiRouter(ch *currency.Handler, wh *webhook.Handler, th *ticker.Handler, hyperHandler *hyperliquid.Handler) *ApiRouter {
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
		h.GET("/whales", api.hyperHandler.WhaleLeaderboardGet())
	}

	base.POST("/webhook", middleware.RequestValidationMiddleware(), api.wh.HandlerWebhook())
}
