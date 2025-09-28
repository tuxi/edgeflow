package instrument

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
	service service.InstrumentService
}

func NewHandler(service service.InstrumentService) *Handler {
	handler := &Handler{service: service}

	return handler
}

func (h *Handler) CoinsGetList() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.CurrencyListReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		exId, err := strconv.ParseInt(req.ExId, 10, 64)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "exchancge id转换错误"), nil)
			return
		}

		// 校验参数
		if req.Limit < 0 {
			req.Limit = 2
		}
		if req.Page < 1 {
			req.Page = 1
		}

		res, err := h.service.InstrumentsListGetByExchange(ctx, exId, req.Page, req.Limit)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}
