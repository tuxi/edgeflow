package webhook

import (
	"fmt"
	"testing"
)

func TestWebhookHandler_HandleWebhook(t *testing.T) {
	str := FormatTVSymbol("BTC/USDC")
	fmt.Println(str) // BTC/USDC
}
