package webhook

import (
	"fmt"
	"testing"
)

func TestWebhookHandler_HandleWebhook(t *testing.T) {
	str := FormatSymbol("BTC/USDC")
	fmt.Println(str) // BTC/USDC
}
