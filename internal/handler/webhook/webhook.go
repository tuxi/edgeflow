package webhook

import (
	"edgeflow/internal/position"
	"edgeflow/internal/service"
	"edgeflow/internal/signal"
	"edgeflow/internal/strategy"
	"edgeflow/internal/webhook"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
	"log"
)

type Handler struct {
	dispatcher *strategy.StrategyDispatcher
	rc         *service.RiskService
	sm         signal.Manager
	ps         *position.PositionService
	whHandler  *webhook.WebhookHandler
}

func NewHandler(
	d *strategy.StrategyDispatcher,
	rc *service.RiskService,
	sm signal.Manager,
	ps *position.PositionService) *Handler {
	h := &Handler{
		dispatcher: d,
		rc:         rc,
		sm:         sm,
		ps:         ps,
	}
	h.whHandler = webhook.NewWebhookHandler(d, rc, sm, ps)
	return h
}

func (h *Handler) HandlerWebhook() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		h.whHandler.Handle(ctx.Request, func(err error, statusCode int) {
			if err != nil {
				response.JSON(ctx, errors.Wrap(err, statusCode, "接口调用失败"), nil)
			} else {
				response.JSON(ctx, nil, nil)
				log.Printf("Signal received")
			}
		})
	}
}
