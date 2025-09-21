package router

import (
	"edgeflow/internal/handler/coin"
	"edgeflow/internal/handler/webhook"
	"edgeflow/internal/middleware"
	"github.com/gin-gonic/gin"
)

type ApiRouter struct {
	coinHandler *coin.Handler
	wh          *webhook.Handler
}

func NewApiRouter(ch *coin.Handler, wh *webhook.Handler) *ApiRouter {
	return &ApiRouter{coinHandler: ch, wh: wh}
}

func (api *ApiRouter) Load(g *gin.Engine) {

	// auth
	base := g.Group("/api/v1")

	c := base.Group("/coins", middleware.RequestValidationMiddleware())
	{
		// 获取币种列表
		c.GET("/list", api.coinHandler.CoinsGetList())
	}

	base.POST("/webhook", middleware.RequestValidationMiddleware(), api.wh.HandlerWebhook())
}
