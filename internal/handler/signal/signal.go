package signal

import (
	"edgeflow/internal/model"
	"edgeflow/internal/service/signal"
	"edgeflow/pkg/errors"
	"edgeflow/pkg/errors/ecode"
	"edgeflow/pkg/response"
	"github.com/gin-gonic/gin"
)

type SignalHandler struct {
	signalService *signal.SignalProcessorService
}

func NewSignalHandler(signalService *signal.SignalProcessorService) *SignalHandler {
	return &SignalHandler{
		signalService: signalService,
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
		res, err := sh.signalService.SignalGetDetail(ctx, req.SignalID)
		if err != nil {
			response.JSON(ctx, errors.Wrap(err, ecode.Unknown, "接口调用失败"), nil)
		} else {
			response.JSON(ctx, nil, res)
		}
	}
}
