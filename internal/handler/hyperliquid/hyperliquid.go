package hyperliquid

import (
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service *service.HyperLiquidService
}

func NewHandler(service *service.HyperLiquidService) *Handler {
	return &Handler{
		service: service,
	}
}

func (h *Handler) WhaleLeaderboardGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var req model.HyperWhaleLeaderBoardReq
		if err := ctx.ShouldBind(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := h.service.GetTopWhales(ctx, req.Limit, req.DatePeriod, req.FilterPeriod)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

// 获取鲸鱼的账户信息：包含仓位
func (h *Handler) WhaleAccountSummaryGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.HyperWhaleAccountReq
		if err := ctx.ShouldBind(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := h.service.WhaleAccountSummaryGet(ctx, req.Address)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

// 根据地址获取鲸鱼信息
func (h *Handler) WhaleInfoGetByAddress() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req struct {
			Address string `form:"address" json:"address"`
		}

		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := h.service.GetWhalePortfolioInfoGetAddress(ctx, req.Address)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

func (h *Handler) WhaleUserFillOrderHistoryGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.HyperWhaleFillOrdersReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		if req.Address == "" {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "address 不能为空"), nil)
			return
		}

		res, err := h.service.WhaleUserFillOrdersHistory(ctx, req)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

func (h *Handler) WhaleUserOpenOrderHistoryGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.HyperWhaleOpenOrdersReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		if req.Address == "" {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "address 不能为空"), nil)
			return
		}

		res, err := h.service.WhaleUserOpenOrdersHistory(ctx, req.Address)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

func (h *Handler) WhaleUserNonFundingLedgerGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.WhaleUserNonFundingLedgerReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		if req.Address == "" {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "address 不能为空"), nil)
			return
		}

		res, err := h.service.WhaleUserNonFundingLedgerGet(ctx, req.Address)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

func (h *Handler) TopWhalePositionsGet() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var req model.WhalePositionFilterReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		res, err := h.service.GetTopWhalePositions(ctx, req)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

func (h *Handler) TopWhalePositionsAnalyze() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		res, err := h.service.AnalyzeTopPositions(ctx)
		if err != nil {
			response.JSON(ctx, err, nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

// GetCustomWinRateLeaderboardHandler: 核心竞争力功能 Handler
// 路径: GET /api/v1/whale-alpha-rank?limit=N
func (h *Handler) GetCustomWinRateLeaderboardHandler() gin.HandlerFunc {
	return func(ctx *gin.Context) {

		var req model.CustomLeaderboardReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		// 设定最大限制，避免用户请求过大数据量导致 Redis 负载过高
		if req.Limit > 500 {
			req.Limit = 500
		}

		// 将直接从 Redis ZSET 中查询数据
		leaderboard, err := h.service.GetWinRateLeaderboardFromDB(ctx, req.Limit)
		if err != nil {
			response.JSON(ctx, err, nil)
			return
		}

		response.JSON(ctx, nil, leaderboard)
	}
}
