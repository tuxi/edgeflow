package trend

import (
	model2 "edgeflow/internal/model"
	"encoding/csv"
	"fmt"
	"github.com/nntaoli-project/goex/v2/model"
	"os"
	"strconv"
	"strings"
)

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
		t := k.Timestamp.Format("2006-01-02 15:04:05")

		record := []string{
			strconv.FormatInt(k.Timestamp.UnixMilli(), 10),
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

func extractCloses(klines []model2.Kline) []float64 {
	closes := make([]float64, len(klines))
	for i, k := range klines {
		closes[i] = k.Close
	}
	return closes
}
