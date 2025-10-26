package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/service"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/exchange"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
	"strconv"
)

type SignalHandler struct {
	signalService *service.SignalProcessorService
	ex            exchange.Exchange
}

func NewSignalHandler(signalService *service.SignalProcessorService, ex exchange.Exchange) *SignalHandler {
	return &SignalHandler{
		signalService: signalService,
		ex:            ex,
	}
}

func (sh *SignalHandler) SignalGetList() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		list, err := sh.signalService.SignalGetList(ctx)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.NotFoundErr, err.Error()), nil)
			return
		}
		response.JSON(ctx, nil, list)
	}
}

func (sh *SignalHandler) GetSignalDetailByID() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.SignalDetailReq
		if err := ctx.ShouldBindQuery(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		sid, err := strconv.ParseInt(req.SignalID, 10, 64)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "SignalID转换错误"), nil)
			return
		}

		res, err := sh.signalService.SignalGetDetail(ctx, sid)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}

// 执行信号
func (h *SignalHandler) ExecuteSignal() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		var req model.SignalExecutionReq
		if err := ctx.ShouldBindBodyWithJSON(&req); err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, err.Error()), nil)
			return
		}

		sid, err := strconv.ParseInt(req.SignalID, 10, 64)
		if err != nil {
			response.JSON(ctx, errors.WithCode(ecode.ValidateErr, "SignalID转换错误"), nil)
			return
		}

		err = h.signalService.ExecuteOrder(ctx, sid, h.ex)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, nil)
		}
	}
}
