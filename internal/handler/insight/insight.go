package insight

import (
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	apierrors "edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *service.InsightService
}

func NewHandler(service *service.InsightService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) MarketOverviewGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		res, err := h.service.GetMarketOverview(ctx, ctx.Query("locale"))
		response.JSON(ctx, err, res)
	}
}

func (h *Handler) MarketWatchlistGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.MarketWatchlistReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, apierrors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}
		res, err := h.service.GetMarketWatchlist(ctx, req)
		response.JSON(ctx, err, res)
	}
}

func (h *Handler) AssetSummaryGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		instrumentID := ctx.Param("instrument_id")
		if instrumentID == "" {
			response.JSON(ctx, apierrors.WithCode(ecode.ValidateErr, "instrument_id is required"), nil)
			return
		}

		locale := ctx.Query("locale")
		res, err := h.service.GetAssetSummary(ctx, instrumentID, locale)
		response.JSON(ctx, err, res)
	}
}

func (h *Handler) AssetTimelineGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		instrumentID := ctx.Param("instrument_id")
		if instrumentID == "" {
			response.JSON(ctx, apierrors.WithCode(ecode.ValidateErr, "instrument_id is required"), nil)
			return
		}

		var req model.AssetTimelineReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, apierrors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := h.service.GetAssetTimeline(ctx, instrumentID, req.Period, req.Limit, req.Locale)
		response.JSON(ctx, err, res)
	}
}

func (h *Handler) AssetDigestGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		instrumentID := ctx.Param("instrument_id")
		if instrumentID == "" {
			response.JSON(ctx, apierrors.WithCode(ecode.ValidateErr, "instrument_id is required"), nil)
			return
		}

		var req model.AssetDigestReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, apierrors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := h.service.GetAssetDigest(ctx, instrumentID, req.Limit, req.Locale)
		response.JSON(ctx, err, res)
	}
}
