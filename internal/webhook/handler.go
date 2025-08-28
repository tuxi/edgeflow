package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"edgeflow/internal/config"
	"edgeflow/internal/position"
	"edgeflow/internal/service"
	"edgeflow/internal/signal"
	"edgeflow/internal/strategy"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
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

	var sig signal.Signal
	if err := json.Unmarshal(body, &sig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if sig.Symbol == "" {
		http.Error(w, "Invalid JSON empty symbol", http.StatusBadRequest)
		return
	}
	sig.Symbol = FormatTVSymbol(sig.Symbol)
	if sig.Strategy == "" {
		http.Error(w, "Invalid JSON empty strategy", http.StatusBadRequest)
		return
	}
	log.Printf("[Webhook] Received signal: %v+\n", sig)

	if sig.Level > 3 || sig.Level < 1 {
		http.Error(w, "Invalid JSON error level", http.StatusBadRequest)
		return
	}

	// 风控检查，是否允许下单
	err = wh.rc.Allow(context.Background(), sig.Strategy, sig.Symbol, sig.Side, sig.TradeType)
	if err != nil {
		http.Error(w, fmt.Sprintf("触发风控:%v，无法下单，稍后再试", err.Error()), http.StatusBadRequest)
		return
	}

	// STEP 1: 校验信号有效期
	expired := sig.IsExpired()
	if expired {
		log.Println("❌ 信号过期，忽略:", sig)
		http.Error(w, "信号过期，忽略", http.StatusBadRequest)
		return
	}

	// 分发策略
	wh.dispatcher.Dispatch(sig, func(err error) {
		// 在信号执行完毕后缓存信号
		wh.sm.Save(sig)

		if err != nil {
			http.Error(w, fmt.Sprintf("%s", err), http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, "Signal received")
		}
	})
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

// FormatTVSymbol 将 TradingView ticker 转换为服务端可识别的 symbol
func FormatTVSymbol(tvSymbol string) string {
	// 后缀 quote 币种列表
	quotes := []string{"USDT", "USD", "USDC"}

	for _, q := range quotes {
		if strings.HasSuffix(tvSymbol, q) {
			base := strings.TrimSuffix(tvSymbol, q)

			if strings.HasSuffix(base, "/") {
				return base + q
			}
			return base + "/" + q
		}
	}
	// 没匹配到就返回原始值
	return tvSymbol
}
