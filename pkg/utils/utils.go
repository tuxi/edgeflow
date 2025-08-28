package utils

import (
	"fmt"
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

/*

 */
