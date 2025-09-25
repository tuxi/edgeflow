package hype

import (
	"context"
	"edgeflow/pkg/hype/rest"
	"edgeflow/pkg/hype/stream"
	"fmt"
	"log"
	"testing"
)

func Test_stream(t *testing.T) {
	// restClient, _ := rest.NewHyperliquidRestClient(
	// 	"https://api.hyperliquid.xyz",
	// 	"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	// )

	ethAddress := "0x880ac484a1743862989a441d6d867238c7aa311c"

	websocketClient, _ := stream.NewHyperliquidWebsocketClient("wss://api.hyperliquid.xyz/ws", func(client *stream.HypeliquidWebsocketClient) {

	})
	webSocketErr := websocketClient.StreamOrderUpdates(ethAddress)
	if webSocketErr != nil {
		fmt.Println("Error streaming all mids:", webSocketErr)
		return
	}
	websocketClient.StreamAllMids()

	go func() {
		for range websocketClient.OrderChan {
			fmt.Println("x")
		}
	}()

	go func() {
		for range websocketClient.AllMidsChan {
			fmt.Println("y")
		}
	}()

	for e := range websocketClient.ErrorChan {
		fmt.Println(e)
	}
}

func Test_Rest(t *testing.T) {
	client, err := rest.NewHyperliquidRestClient("https://api.hyperliquid.xyz", "https://stats-data.hyperliquid.xyz/Mainnet/leaderboard")
	if err != nil {
		// handle error
		log.Printf("初始化client报错：%v", err)
	}

	user := "0xa715dbb56dc8c1a6a2d84bfcb0f45bf4da07ad14"
	accountSummary, err := client.PerpetualsAccountSummary(context.Background(), user)
	if err != nil {
		// handle error
		log.Printf("PerpetualsAccountSummary err: %v", err)
		return
	}

	log.Printf("当前仓位 : %v", accountSummary.AssetPositions)
}

func Test_leaderboard(t *testing.T) {
	restClient, _ := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	//data, err := restClient.LeaderboardCall()
	//if err != nil {
	//	panic(err)
	//}
	//log.Println(data)

	items, err := restClient.UserNonFundingLedgerUpdates(context.Background(), "0x880ac484a1743862989a441d6d867238c7aa311c")
	if err != nil {
		panic(err)
	}
	log.Println(items)
}
