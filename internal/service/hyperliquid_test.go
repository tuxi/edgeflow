package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

func TestHyperLiquid(t *testing.T) {
	url := "https://api.hyperliquid.xyz/info"

	reqBody := map[string]interface{}{
		"type": "predictedFundings",
		//"user": "0xa715dbb56dc8c1a6a2d84bfcb0f45bf4da07ad14",
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
