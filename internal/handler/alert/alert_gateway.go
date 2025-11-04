package alert

import (
	"context"
	"edgeflow/internal/service"
	"edgeflow/pkg/kafka"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// keepalive的ping间隔
const pingPeriod = 30 * time.Second
const pongWait = 60 * time.Second

// client send buffer
const sendBufSize = 1024

// AlertGateway 管理 alert websocket 连接并从 AlertService 订阅消息
type AlertGateway struct {
	service  *service.AlertService
	consumer kafka.ConsumerService // Kafka Consumer
	// 使用 RWMutex 保护普通 Map
	mu      sync.RWMutex
	clients map[string]*AlertClientConn // map[clientID]*AlertClientConn

	upgrader websocket.Upgrader
}

func NewAlertGateway(svc *service.AlertService, consumer kafka.ConsumerService) *AlertGateway {
	g := &AlertGateway{
		service:  svc,
		consumer: consumer,
		mu:       sync.RWMutex{},
		clients:  make(map[string]*AlertClientConn),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}

	// 启动监听 Broadcast (TopicSystem)
	go g.listenForBroadcasts()

	// 🚀 启动监听定向推送 (新的 Kafka Topic)
	go g.listenForDevicePushes()

	return g
}

// ServeWS 建立 websocket 连接
func (g *AlertGateway) ServeWS(c *gin.Context) {
	clientID := c.Query("client_id")
	if clientID == "" {
		// 要求 client_id
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := g.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("AlertGateway upgrade error:", err)
		return
	}

	client := &AlertClientConn{
		ClientID: clientID,
		Conn:     conn,
		Send:     make(chan []byte, sendBufSize),
	}

	// 使用读写锁确保原子替换
	var oldClient *AlertClientConn
	g.mu.Lock()
	{
		// 1. 检查是否存在旧连接
		if existing, ok := g.clients[clientID]; ok {
			oldClient = existing
			oldClient.replaced = true // 标记旧连接
			log.Printf("AlertGateway: client %s reconnected, marking old connection as replaced.", clientID)
		}

		// 2. 存入新连接
		g.clients[clientID] = client
	}
	g.mu.Unlock()

	// 3. 异步关闭旧连接
	if oldClient != nil {
		// 异步关闭，防止阻塞ServeWS
		go oldClient.Close()
		log.Printf("AlertGateway: closed old connection for %s", clientID)
	}

	defer func() {
		// 从活跃 clients map 中移除（仅在未被替换时）
		g.mu.Lock()
		{
			// 再次检查，确保只有当前的 client 才能被移除
			if current, ok := g.clients[clientID]; ok && current == client {
				delete(g.clients, clientID)
				log.Printf("AlertGateway: removed client %s from active map.", clientID)
			} else {
				log.Printf("AlertGateway: defer remove skipped for %s (replaced or already removed).", clientID)
			}
		}
		g.mu.Unlock()

		// 无论如何，确保本 client 的资源被关闭
		client.Close()
	}()

	// 启动 writePump
	go client.writePump()

	// ReadPump 阻塞直到客户端关闭
	client.readPump(g)
}

// 监听全量广播
func (g *AlertGateway) listenForBroadcasts() {
	alertCh, err := g.consumer.Consume(context.Background(), kafka.TopicAlertSystem, "edgeflow_alert_gateway_group")
	if err != nil {
		log.Fatalf("未能启动Alert的kafka消费者： %v", err)
	}
	//ch := g.service.SubscribeBroadcast()
	for msg := range alertCh {
		g.broadcast(msg.Value)
	}
}

// 监听定向推送 Topic
func (g *AlertGateway) listenForDevicePushes() {
	alertCh, err := g.consumer.Consume(context.Background(), kafka.TopicAlertDirect, "edgeflow_alert_direct_group")
	if err != nil {
		log.Fatalf("AlertGateway 未能启动 Alert 定向推送 Kafka 消费者：%v", err)
	}

	for msg := range alertCh {
		// kafka key 就是deviceID
		deviceID := string(msg.Key)
		g.sendToDevice(deviceID, msg.Value)
	}
}

// broadcast 全量广播
func (g *AlertGateway) broadcast(data []byte) {
	g.mu.RLock()
	// 遍历 Map 需要在锁的保护下
	clientsCopy := make([]*AlertClientConn, 0, len(g.clients))
	for _, c := range g.clients {
		clientsCopy = append(clientsCopy, c)
	}
	g.mu.RUnlock()

	// 在解锁后对副本进行操作
	for _, c := range clientsCopy {
		c.safeSend(data)
	}
}

// sendToDevice 定向发送（若在线）
func (g *AlertGateway) sendToDevice(deviceId string, data []byte) bool {
	g.mu.RLock()
	c, ok := g.clients[deviceId]
	g.mu.RUnlock()

	if ok {
		return c.safeSend(data) // 内部安全发送
	}
	return false
}
