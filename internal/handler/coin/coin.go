package coin

import (
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
)

type Handler struct {
	service service.CoinService
}

func NewHandler(service service.CoinService) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CoinsGetList() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.CoinListReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
		}

		caregoryId, err := strconv.ParseInt(req.CategoryId, 10, 64)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "category id转换错误"), nil)
			return
		}
		res, err := h.service.CoinListGetByCategory(ctx, caregoryId)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}
