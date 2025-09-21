package utils

import (
	"fmt"
	"github.com/spf13/cast"
	"strconv"
	"strings"
	"time"
)

// Retry 尝试执行 fn，如果失败则重试，最多 retries 次
// delay 是两次重试之间的间隔，backoff=true 表示指数退避
func Retry(retries int, delay time.Duration, backoff bool, fn func() error) error {
	var err error
	for i := 0; i < retries; i++ {
		err = fn()
		if err == nil {
			return nil
		}

		if i < retries-1 { // 最后一次就不用 sleep 了
			sleep := delay
			if backoff {
				sleep = delay * time.Duration(1<<i) // 1x,2x,4x,8x...
			}
			time.Sleep(sleep)
		}
	}
	return fmt.Errorf("after %d attempts, last error: %w", retries, err)
}

// FormatSymbol 将 TradingView ticker 转换为服务端可识别的 symbol
func FormatSymbol(tvSymbol string) string {
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

// FloatToString 保留的小数点位数,去除末尾多余的0(StripTrailingZeros)
func FloatToString(v float64, n int) string {
	ret := strconv.FormatFloat(v, 'f', n, 64)
	return strconv.FormatFloat(cast.ToFloat64(ret), 'f', -1, 64) //StripTrailingZeros
}
