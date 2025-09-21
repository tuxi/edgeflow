package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"edgeflow/conf"
	"edgeflow/internal/position"
	"edgeflow/internal/service"
	"edgeflow/internal/signal"
	"edgeflow/internal/strategy"
	"edgeflow/pkg/utils"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type WebhookHandler struct {
	dispatcher *strategy.StrategyDispatcher
	rc         *service.RiskService
	sm         signal.Manager
	ps         *position.PositionService
}

func NewWebhookHandler(
	d *strategy.StrategyDispatcher,
	rc *service.RiskService,
	sm signal.Manager,
	ps *position.PositionService) *WebhookHandler {
	return &WebhookHandler{
		dispatcher: d,
		rc:         rc,
		sm:         sm,
		ps:         ps,
	}
}

// TradingView Webhook 的接收器

// HandleWebhook 接收POST 请求并解析为策略信号
func (wh *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {

	wh.Handle(r, func(err error, statusCode int) {
		if err != nil {
			http.Error(w, fmt.Sprintf("%s", err), statusCode)
		} else {
			w.WriteHeader(statusCode)
			fmt.Fprintf(w, "Signal received")
		}
	})
}

// HandleWebhook 接收POST 请求并解析为策略信号
func (wh *WebhookHandler) Handle(r *http.Request, callback func(err error, statusCode int)) {

	// 获取签名
	signature := r.Header.Get("X-Signature")
	if signature == "" {
		callback(fmt.Errorf("missing signature"), http.StatusUnauthorized)
	}

	if r.Method != http.MethodPost {
		callback(fmt.Errorf("only POST allow"), http.StatusMethodNotAllowed)
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		callback(fmt.Errorf("failed to read body"), http.StatusBadRequest)
	}
	defer r.Body.Close()

	// 验签
	if !verifySignature(body, signature) {
		callback(fmt.Errorf("invalid signature"), http.StatusMethodNotAllowed)
	}

	var sig signal.Signal
	if err := json.Unmarshal(body, &sig); err != nil {
		callback(fmt.Errorf("invalid JSON"), http.StatusBadRequest)
	}
	if sig.Symbol == "" {
		callback(fmt.Errorf("invalid JSON empty symbol"), http.StatusBadRequest)
	}
	sig.Symbol = utils.FormatSymbol(sig.Symbol)
	if sig.Strategy == "" {
		callback(fmt.Errorf("invalid JSON empty strategy"), http.StatusBadRequest)
	}
	log.Printf("[Webhook] Received signal: %v+\n", sig)

	if sig.Level > 3 || sig.Level < 1 {
		callback(fmt.Errorf("invalid JSON error level"), http.StatusBadRequest)
	}

	// 风控检查，是否允许下单
	err = wh.rc.Allow(context.Background(), sig.Strategy, sig.Symbol, sig.Side, sig.TradeType)
	if err != nil {
		callback(fmt.Errorf("触发风控:%v，无法下单，稍后再试", err.Error()), http.StatusBadRequest)
	}

	// STEP 1: 校验信号有效期
	expired := sig.IsExpired()
	if expired {
		callback(fmt.Errorf("信号过期，忽略:%v", sig), http.StatusBadRequest)
	}

	// 分发策略
	wh.dispatcher.Dispatch(sig, func(err error) {
		if err != nil {
			callback(err, http.StatusBadRequest)
		} else {
			// 在信号执行完毕后缓存信号
			wh.sm.Save(sig)
			callback(nil, http.StatusOK)
		}
	})
}

func verifySignature(body []byte, signatureHeader string) bool {
	secret := conf.AppConfig.Webhook.Secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedMAC := h.Sum(nil)
	providedMAC, err := hex.DecodeString(signatureHeader)
	if err != nil {
		return false
	}
	return hmac.Equal(providedMAC, expectedMAC)
}
