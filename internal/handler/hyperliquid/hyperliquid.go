package hyperliquid

import (
	"edgeflow/internal/service"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
	"time"
)

type Handler struct {
	service *service.HyperLiquidService
}

func NewHandler(service *service.HyperLiquidService) *Handler {
	service.StartLeaderboardUpdater(time.Minute * 1)
	return &Handler{
		service: service,
	}
}

func (h *Handler) WhaleLeaderboardGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		res, err := h.service.GetTopWhales(ctx, 100)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}

	}
}
