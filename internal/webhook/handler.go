package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"edgeflow/internal/config"
	"edgeflow/internal/model"
	"edgeflow/internal/risk"
	"edgeflow/internal/strategy"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
)

type WebhookHandler struct {
	dispatcher *strategy.StrategyDispatcher
	rc         *risk.RiskControl
}

func NewWebhookHandler(d *strategy.StrategyDispatcher, rc *risk.RiskControl) *WebhookHandler {
	return &WebhookHandler{
		dispatcher: d,
		rc:         rc,
	}
}

// TradingView Webhook 的接收器

// HandleWebhook 接收POST 请求并解析为策略信号
func (wh *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {

	// 获取签名
	signature := r.Header.Get("X-Signature")
	if signature == "" {
		http.Error(w, "Missing signature", http.StatusUnauthorized)
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allow", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		fmt.Fprintln(w, "Failed to read body")
		return
	}
	defer r.Body.Close()

	// 验签
	if !verifySignature(body, signature) {
		http.Error(w, "Invalid signature", http.StatusMethodNotAllowed)
		fmt.Fprintln(w, "Invalid signature")
		return
	}

	var sig model.Signal
	if err := json.Unmarshal(body, &sig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if sig.Strategy == "" {
		http.Error(w, "Invalid JSON empty strategy", http.StatusBadRequest)
		return
	}
	log.Printf("[Webhook] Received signal: %v+\n", sig)

	// 风控检查，是否允许下单
	err = wh.rc.Allow(context.Background(), sig.Strategy, sig.Symbol, sig.Side, sig.TradeType)
	if err != nil {
		http.Error(w, fmt.Sprintf("触发风控:%v，无法下单，稍后再试", err.Error()), http.StatusBadRequest)
		return
	}

	err = wh.handleSignal(sig)
	if err != nil {
		http.Error(w, "Unkonw strategy", http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Signal received")
	}
}

func (wh *WebhookHandler) handleSignal(sig model.Signal) error {

	// STEP 1: 校验信号有效期
	expired := sig.IsExpired()
	if expired {
		log.Println("❌ 信号过期，忽略:", sig)
		return errors.New("信号过期，忽略")
	}

	return wh.dispatcher.Dispatch(sig)
}

func verifySignature(body []byte, signatureHeader string) bool {
	secret := config.AppConfig.Webhook.Secret

	h := hmac.New(sha256.New, []byte(secret))
	h.Write(body)
	expectedMAC := h.Sum(nil)
	providedMAC, err := hex.DecodeString(signatureHeader)
	if err != nil {
		return false
	}
	return hmac.Equal(providedMAC, expectedMAC)
}
