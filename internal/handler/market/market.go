package market

import (
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
)

type MarketHandler struct {
	marketService *service.MarketDataService
}

func NewMarketHandler(marketService *service.MarketDataService) *MarketHandler {
	return &MarketHandler{
		marketService: marketService,
	}
}

func (h *MarketHandler) SortedInstIDsGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		currentIDs, sortBy := h.marketService.GetSortedIDsl()
		var data = struct {
			SortBy  string   `json:"sort_by"`
			InstIds []string `json:"inst_ids"`
		}{
			SortBy:  sortBy,
			InstIds: currentIDs,
		}
		response.JSON(ctx, nil, data)
	}
}

func (m *MarketHandler) GetDetail() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.MarketDetailReq
		if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := m.marketService.GetDetailByID(ctx, req)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}
