package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"edgeflow/internal/config"
	"edgeflow/internal/strategy"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// TradingView Webhook 的接收器

type SignalPayload struct {
	Symbol   string `json:"symbol"`
	Side     string `json:"side"`     // buy / sell
	Strategy string `json:"strategy"` // 策略名
	Comment  string `json:"comment"`  // 可选策略说明
}

// HandleWebhook 接收POST 请求并解析为策略信号
func HandleWebhook(w http.ResponseWriter, r *http.Request) {

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
		return
	}
	defer r.Body.Close()

	// 验签
	if !verifySignature(body, signature) {
		http.Error(w, "Invalid signature", http.StatusMethodNotAllowed)
		return
	}

	var payload SignalPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[Webhook] Received signal: %v+\n", payload)

	// TODO: 分发给策略执行器
	executor, err := strategy.Get(payload.Strategy)
	if err != nil {
		http.Error(w, "Unkonw strategy", http.StatusBadRequest)
		return
	}
	params := strategy.ExecutionParams{
		Symbol:  payload.Symbol,
		Side:    payload.Side,
		Comment: payload.Comment,
	}

	go executor.Execute(r.Context(), params)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Signal received")
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
