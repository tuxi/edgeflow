package signal

import (
	"edgeflow/internal/service/signal"
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

	}
}
