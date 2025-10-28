package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
	"time"
)

// 用户收益
func TestHyperLiquid(t *testing.T) {
	url := "https://api.hyperliquid.xyz/info"

	reqBody := map[string]interface{}{
		"type":      "portfolio", // predictedFundings
		"user":      "0x53babe76166eae33c861aeddf9ce89af20311cd0",
		"startTime": time.Now().UnixNano(),
		//"endTime": 1700000000,
	}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}

// 资金费率
func TestHyperLiquidPredictedFundings(t *testing.T) {
	url := "https://api.hyperliquid.xyz/info"

	reqBody := map[string]interface{}{
		"type": "predictedFundings",
		//"user": "0xa715dbb56dc8c1a6a2d84bfcb0f45bf4da07ad14",
		//"startTime": 1690000000,  // 可选：时间范围
		//"endTime": 1700000000,
	}
	data, _ := json.Marshal(reqBody)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(data))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))
}
