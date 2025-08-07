package webhook

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"edgeflow/internal/config"
	"edgeflow/internal/model"
	"edgeflow/internal/strategy"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// TradingView Webhook 的接收器

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

	var sig model.Signal
	if err := json.Unmarshal(body, &sig); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	log.Printf("[Webhook] Received signal: %v+\n", sig)

	err = handleSignal(sig)
	if err != nil {
		http.Error(w, "Unkonw strategy", http.StatusBadRequest)
	} else {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "Signal received")
	}
}

func handleSignal(sig model.Signal) error {

	// STEP 1: 校验信号有效期
	expired := sig.IsExpired()
	if expired {
		log.Println("❌ 信号过期，忽略:", sig)
		return errors.New("信号过期，忽略")
	}

	// STEP 2: 获取最新缓存
	lastSignals := model.SignalCache.Latest[sig.Symbol]

	// STEP 3: 不同等级的处理逻辑
	switch sig.Level {
	case 1:
		dispatch(sig, false) // 分发指标
	case 2:
		lvl1, hasLvl1 := lastSignals[1]
		if hasLvl1 && lvl1.Side == sig.Side {
			dispatch(sig, false) // 与 L1 一致，执行
		} else if hasLvl1 && lvl1.Side != sig.Side {
			dispatch(sig, true) // 方向冲突，清仓
		} else {
			log.Println("等待L1方向，L2信号延迟执行")
		}
	case 3:
		lvl2, hasLvl2 := lastSignals[2]
		lvl1, hasLvl1 := lastSignals[1]
		lv3, hasLv3 := lastSignals[3]
		if hasLvl2 && lvl2.Side == sig.Side {
			if hasLvl1 && lvl1.Side == sig.Side {
				dispatch(sig, false)
			} else {
				log.Println("L3 信号仅记录，不执行")
			}
		} else if hasLv3 && lv3.Side == sig.Side {
			dispatch(sig, false)
		} else {
			log.Println("L3 信号仅记录，不执行")
		}
	}

	cacheSignal(sig)

	return nil
}

// TODO: 分发给策略执行器
func dispatch(sig model.Signal, isClose bool) error {
	executor, err := strategy.Get(sig.Strategy)
	if err != nil {
		fmt.Printf("未知指标：%v", sig.Strategy)
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	go func() {
		defer cancel()
		if isClose {
			executor.ClosePosition(ctx, sig)
		} else {
			executor.Execute(ctx, sig)
		}

	}()
	return nil
}

// 缓存策略
func cacheSignal(sig model.Signal) {
	if _, ok := model.SignalCache.Latest[sig.Symbol]; !ok {
		model.SignalCache.Latest[sig.Symbol] = make(map[int]model.Signal)
	}
	model.SignalCache.Latest[sig.Symbol][sig.Level] = sig
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
