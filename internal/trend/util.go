package trend

import (
	model2 "edgeflow/internal/model"
	"encoding/csv"
	"fmt"
	"github.com/nntaoli-project/goex/v2/model"
	"os"
	"strconv"
	"strings"
	"time"
)

// 判断最前面那根是否未收盘（以周期判断）
func isUnclosedBar(tsMillis int64, tf time.Duration, now time.Time) bool {
	barStart := time.UnixMilli(tsMillis)
	barEnd := barStart.Add(tf)
	return now.Before(barEnd) // 还没到收盘时间 → 未收盘
}

// 将最新在前的切片，转换为 从旧到新（必要时丢弃未收盘）
// latestFirst 是从okx获取的原始k线数组，顺序是从新到旧
// dropUnclosed 是否丢掉未收盘的当前bar
func normalizeCandles(latestFirst []model2.Kline, period model.KlinePeriod, dropUnclosed bool) []model2.Kline {
	if len(latestFirst) == 0 {
		return nil
	}
	tf := periodToDuration(period)
	start := 0
	if dropUnclosed && isUnclosedBar(latestFirst[0].Timestamp, tf, time.Now()) {
		start = 1 // 丢掉正在形成的那根
	}
	if start >= len(latestFirst) {
		return nil
	}
	// 反转为 从旧到新
	n := len(latestFirst) - start
	out := make([]model2.Kline, n)
	for i := 0; i < n; i++ {
		out[i] = latestFirst[len(latestFirst)-1-i] // 最旧 → 最前
	}
	return out
}

func periodToDuration(p model.KlinePeriod) time.Duration {
	switch p {
	case model.Kline_1min:
		return time.Minute
	case model.Kline_5min:
		return 5 * time.Minute
	case model.Kline_15min:
		return 15 * time.Minute
	case model.Kline_30min:
		return 30 * time.Minute
	case model.Kline_60min, model.Kline_1h:
		return time.Hour
	case model.Kline_4h:
		return 4 * time.Hour
	case model.Kline_6h:
		return 6 * time.Hour
	case model.Kline_1day:
		return 24 * time.Hour
	case model.Kline_1week:
		return 7 * 24 * time.Hour
	default:
		return 0
	}
}

func containsSignal(list []TrendState, side TrendDirection) bool {
	for _, s := range list {
		if s.Direction == side {
			return true
		}
	}
	return false
}

func genCSV(symbol string, interval model.KlinePeriod, lines []model2.Kline) {
	sy := symbol
	if strings.Contains(symbol, "/") {
		sy = strings.ReplaceAll(symbol, "/", "")
	}

	path := fmt.Sprintf("logs/%v_%v.csv", sy, interval)
	file, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入 CSV 头
	writer.Write([]string{"timestamp", "datetime", "open", "high", "low", "close", "volume"})

	for _, k := range lines {
		// 转换毫秒时间戳为可读日期
		t := time.UnixMilli(k.Timestamp).Format("2006-01-02 15:04:05")

		record := []string{
			strconv.FormatInt(k.Timestamp, 10),
			t,
			fmt.Sprintf("%.2f", k.Open),
			fmt.Sprintf("%.2f", k.High),
			fmt.Sprintf("%.2f", k.Low),
			fmt.Sprintf("%.2f", k.Close),
			fmt.Sprintf("%.4f", k.Vol),
		}

		writer.Write(record)
	}

}
