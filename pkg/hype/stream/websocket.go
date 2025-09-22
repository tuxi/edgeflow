package stream

import (
	"edgeflow/pkg/hype/types"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type HypeliquidWebsocketClient struct {
	websocketUrl string
	conn         *websocket.Conn
	AllMidsChan  chan map[string]float64 // 价格变化的通道
	OrderChan    chan []types.Order      // 订单变化的通道
	ErrorChan    chan error              // 错误通道
	mutex        sync.Mutex
	lastRequest  time.Time

	ConnectSuccess func(*HypeliquidWebsocketClient)
}

func NewHyperliquidWebsocketClient(rawUrl string, connectSuccess func(*HypeliquidWebsocketClient)) (*HypeliquidWebsocketClient, error) {
	if _, err := url.ParseRequestURI(rawUrl); err != nil {
		return nil, errors.New("Invalid websocket URL")
	}
	conn, _, err := websocket.DefaultDialer.Dial(rawUrl, nil)
	if err != nil {
		return nil, err
	}

	client := &HypeliquidWebsocketClient{
		websocketUrl: rawUrl,
		conn:         conn,
		AllMidsChan:  make(chan map[string]float64),
		OrderChan:    make(chan []types.Order),
		ErrorChan:    make(chan error),
	}
	client.ConnectSuccess = connectSuccess

	// 定时 ping
	go func() {
		ticker := time.NewTicker(50 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			client.mutex.Lock()
			pingMessage := map[string]interface{}{
				"method": "ping",
			}
			if err := client.sendMessage(pingMessage); err != nil {
				fmt.Println("Error sending ping:", err)
				client.ErrorChan <- err
				client.reconnect() // ping 失败也触发重连
			}
			client.mutex.Unlock()
		}
	}()

	if connectSuccess != nil {
		connectSuccess(client)
	}

	// 启动读协程
	go client.readLoop()

	return client, nil
}

func (c *HypeliquidWebsocketClient) reconnect() {
	// 关闭旧的连接
	_ = c.conn.Close()

	for {
		fmt.Println("HypeliquidWebsocketClient attempting to reconnect...")

		// 建立新的连接
		conn, _, err := websocket.DefaultDialer.Dial(c.websocketUrl, nil)
		if err != nil {
			fmt.Println("HypeliquidWebsocketClient reconnect failed:", err)
			time.Sleep(5 * time.Second) // 等待后重试
			continue
		}

		fmt.Println("HypeliquidWebsocketClient reconnected successfully!")

		c.mutex.Lock()
		c.conn = conn
		c.mutex.Unlock()

		// 因为重新连接后connect变成了另外一个新的连接，所以这里要回调给调用者，重新订阅
		// 回调重连成功
		if c.ConnectSuccess != nil {
			c.ConnectSuccess(c)
		}

		// 重新启动读协程
		go c.readLoop()
		break
	}
}

func (c *HypeliquidWebsocketClient) readLoop() {
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			fmt.Println("Read error:", err)
			c.ErrorChan <- err
			c.reconnect() // 出错时尝试重连
			return
		}

		var response types.GenericMessage
		if err := json.Unmarshal(msg, &response); err != nil {
			fmt.Println("Unmarshal error:", err)
			c.ErrorChan <- err
			continue
		}

		switch response.Channel {
		case "allMids":
			c.handleAllMidsMessage(msg)
		case "orderUpdates":
			c.handleOrderUpdatesMessage(msg)
		}
	}
}

func (client *HypeliquidWebsocketClient) handleAllMidsMessage(msg json.RawMessage) {
	var midsResponse types.AllMidsMessage
	if err := json.Unmarshal(msg, &midsResponse); err != nil {
		fmt.Println("Unmarshal error:", err)
		client.ErrorChan <- err
		return
	}

	prices := make(map[string]float64)
	for k, v := range midsResponse.Data.Prices {
		if k[0] != '@' {
			floatVal, _ := strconv.ParseFloat(v, 64)
			prices[k] = floatVal
		}
	}

	client.AllMidsChan <- prices
}

func (client *HypeliquidWebsocketClient) handleOrderUpdatesMessage(msg json.RawMessage) {
	var orderResponse types.OrderUpdatesMessage
	if err := json.Unmarshal(msg, &orderResponse); err != nil {
		fmt.Println("Unmarshal error:", err)
		client.ErrorChan <- err
		return
	}

	client.OrderChan <- orderResponse.Data
}

func (client *HypeliquidWebsocketClient) sendMessage(message interface{}) error {
	// Ensure at least 50ms between requests
	timeSinceLastRequest := time.Since(client.lastRequest)
	if timeSinceLastRequest < 50*time.Millisecond {
		time.Sleep(50*time.Millisecond - timeSinceLastRequest)
	}
	client.lastRequest = time.Now()

	return client.conn.WriteJSON(message)
}

func (client *HypeliquidWebsocketClient) StreamAllMids() error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	subscriptionMessage := map[string]interface{}{
		"method": "subscribe",
		"subscription": map[string]interface{}{
			"type": "allMids",
		},
	}
	if err := client.sendMessage(subscriptionMessage); err != nil {
		close(client.AllMidsChan)
		client.ErrorChan <- err
		return fmt.Errorf("Subscription error: %w", err)
	}
	return nil
}

func (client *HypeliquidWebsocketClient) StreamOrderUpdates(userId string) error {
	client.mutex.Lock()
	defer client.mutex.Unlock()

	subscriptionMessage := map[string]interface{}{
		"method": "subscribe",
		"subscription": map[string]interface{}{
			"type": "orderUpdates",
			"user": userId,
		},
	}
	if err := client.sendMessage(subscriptionMessage); err != nil {
		close(client.OrderChan)
		client.ErrorChan <- err
		return fmt.Errorf("Subscription error: %w", err)
	}
	return nil
}
