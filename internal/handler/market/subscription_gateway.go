package market

import (
	"context"
	"edgeflow/internal/service"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"google.golang.org/protobuf/proto"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SubscriptionGateway 负责按需订阅（K线、深度等）的连接管理和定向推送
type SubscriptionGateway struct {
	// 依赖：K线服务 (目前唯一的外部数据源)
	candleClient *service.OKXCandleService
	// 依赖：Kafka Consumer (用于接收 K线等实时数据)
	consumer kafka.ConsumerService

	mu         sync.Mutex
	upgrader   websocket.Upgrader
	cleanupMap sync.Map // 用于宽限期重连

	// 连接管理用到的原子性结构：维护所有活跃的连接实例，用于重连、清理、CoW 替换
	// 当使用全量推送时可以用到，比如ticker中就用到了
	clients atomic.Value // 存储 map[string]*ClientConn
	// 消息过滤和定向推送。根据消息（SubKey）快速查找所有订阅了该消息的客户端。
	// 核心：用于 Kafka 消息过滤的全局订阅映射
	// Key: SubKey (e.g., "CANDLE:BTC-USDT:15m"), Value: *sync.Map (存储订阅该 Key 的 ClientConn)
	subscriptionMap *sync.Map
}

func NewSubscriptionGateway(candleClient *service.OKXCandleService, consumer kafka.ConsumerService) *SubscriptionGateway {
	g := &SubscriptionGateway{
		candleClient: candleClient,
		consumer:     consumer,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		subscriptionMap: &sync.Map{},
	}
	g.clients.Store(make(map[string]*ClientConn))

	// 启动 Kafka 消费和定向推送 (替换 listenForCandleUpdates)
	go g.listenAndFilterUpdates()
	// 启动订阅错误监听 (保留)
	go g.listenForSubscriptionErrors()

	return g
}

// 替换原 MarketHandler.listenForCandleUpdates
func (g *SubscriptionGateway) listenAndFilterUpdates() {
	// 订阅主题：marketdata_subscribe
	subCh, err := g.consumer.Consume(context.Background(), "marketdata_subscribe", "subscription_gateway_group")
	if err != nil {
		log.Fatalf("Failed to start Subscription Kafka consumer: %v", err)
	}

	for msg := range subCh {
		// 1. Protobuf 反序列化，确定订阅键
		var wsMsg pb.WebSocketMessage // 假设 Kafka 写入的是通用 Protobuf 消息
		if err := proto.Unmarshal(msg.Value, &wsMsg); err != nil {
			log.Printf("WARN: Failed to unmarshal Protobuf subscribe message: %v", err)
			continue
		}

		// 2. 构造通用订阅键 (SubKey)
		subKey := getSubKeyFromProtoMessage(&wsMsg)
		if subKey == "" {
			continue // 无法识别的消息类型
		}

		// 3. 执行过滤和定向推送
		if clientsMap, found := g.subscriptionMap.Load(subKey); found {
			clientsMap.(*sync.Map).Range(func(key, value interface{}) bool {
				client := value.(*ClientConn)
				// 直接发送原始 Protobuf 二进制数据
				client.safeSend(msg.Value)
				return true
			})
		}
	}
}

// MarketHandler
func (h *SubscriptionGateway) listenForSubscriptionErrors() {
	// 获取错误通道
	errorCh := h.candleClient.GetErrorChannel()

	for subErr := range errorCh {

		// 1. 构造一个错误消息给客户端
		target_action := subErr.Data["target_action"]
		if target_action != "subscribe_candle" {
			// 这里目前只有k线订阅相关的信息，其他的忽略
			continue
		}
		period := subErr.Data["period"]
		symbol := subErr.Data["symbol"]
		if symbol == "" || period == "" {
			// 这里目前只有k线订阅相关的信息，其他的忽略
			continue
		}

		// 2. 广播给所有客户端（如果您不知道哪个客户端发起的请求）
		// 如果您的业务要求只通知发起请求的客户端，您需要在订阅时记录 clientID/connID

		// 转换为protobuf消息
		payload := &pb.ErrorMessage{
			Action: subErr.Action,
			Data:   subErr.Data,
		}

		potobufMsg := &pb.WebSocketMessage{
			Type:    "CLIENT_ERROR",
			Payload: &pb.WebSocketMessage_ErrorMessage{ErrorMessage: payload},
		}

		data, err := proto.Marshal(potobufMsg)
		if err != nil {
			log.Printf("ERROR: Protobuf 序列化错误消息失败: %v", err)
			continue
		}

		// 定向发送客户端
		subKey := fmt.Sprintf("CANDLE:%s:%s", period, symbol)

		if clientsMap, found := h.subscriptionMap.Load(subKey); found {
			clientsMap.(*sync.Map).Range(func(key, value interface{}) bool {
				client := value.(*ClientConn)
				// 使用 safeSend 发送 Protobuf 二进制数据
				client.safeSend(data)
				return true
			})
		}
	}
}

// 辅助函数：根据 Protobuf 消息提取订阅键
// 根据我们的 Protobuf 定义来实现
func getSubKeyFromProtoMessage(msg *pb.WebSocketMessage) string {
	// 从 K线更新中提取 "CANDLE:BTC-USDT:15m"
	if payload := msg.GetKlineUpdate(); payload != nil {
		// 假设您的 K 线更新消息中包含 Symbol 和 Period
		return fmt.Sprintf("CANDLE:%s:%s", payload.InstId, payload.TimePeriod)
	}
	// TODO: 添加其他频道 (DEPTH, TRADE) 的逻辑
	return ""
}

// ServeWS 仅处理连接建立和断开
func (h *SubscriptionGateway) ServeWS(c *gin.Context) {

	// 获取clientId
	clientID := c.Query("client_id")
	if clientID == "" {
		// 强制要求客户端提供唯一的ID，否则拒绝连接
		// 或者生成一个临时的UUID作为Client ID
		log.Println("客户单缺少client_id 拒绝连接.")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("upgrade error:", err)
		return
	}

	newClient := &ClientConn{
		ClientID:      clientID,
		Conn:          conn,
		Send:          make(chan []byte, 100),
		Subscriptions: make(map[string]struct{}),
	}

	// 收集需要恢复的订阅列表
	var subscriptionsToRestore []string
	var oldClient *ClientConn
	var isFromCleanupMap bool

	// 查找旧连接：先查找活跃的map，后查找cleanup map
	// 从活跃连接中查找
	h.mu.Lock()
	oldClients := h.clients.Load().(map[string]*ClientConn)
	if existingClient, found := oldClients[clientID]; found {
		oldClient = existingClient
	}
	h.mu.Unlock()

	// 如果活跃连接中没有，则检查 cleanupMap (处理宽限期内的重连)
	if oldClient == nil {
		if conn, loaded := h.cleanupMap.Load(clientID); loaded {
			oldClient = conn.(*ClientConn)
			isFromCleanupMap = true
			log.Printf("ClientID %s found in cleanup map. Restoring state from grace period.", clientID)
			// 立即从 cleanupMap 中移除，阻止计时器清理
			h.cleanupMap.Delete(clientID)
		}
	}

	// 执行状态迁移 （前提是找到了旧的连接）
	if oldClient != nil {
		log.Printf("ClientID %s reconnected. Starting state migration.", clientID)

		// 🚨 锁住旧连接的本地状态，执行迁移
		oldClient.mu.Lock()
		// 复制通用的 Subscriptions
		for subKey := range oldClient.Subscriptions {
			newClient.Subscriptions[subKey] = struct{}{}
			subscriptionsToRestore = append(subscriptionsToRestore, subKey)
		}

		// 标记旧连接已被替换，阻止其 defer/cleanup 逻辑执行 Unsubscribe
		oldClient.replaced = true
		// 清空旧的通用订阅
		oldClient.Subscriptions = make(map[string]struct{}, 1) // 不要设置为nil
		oldClient.mu.Unlock()

		log.Printf("ClientID %s: Migrated %d subscriptions to new connection.", clientID, len(subscriptionsToRestore))
	}

	// 执行CoW替换新连接 （原子操作）
	h.mu.Lock()
	{
		// 重新加载最新的活跃连接 map
		oldClients = h.clients.Load().(map[string]*ClientConn)
		newClients := make(map[string]*ClientConn, len(oldClients))

		// 复制旧的 map
		for k, v := range oldClients {
			newClients[k] = v
			subscriptionsToRestore = append(subscriptionsToRestore, k) // 收集 key
		}

		// 替换或添加新连接
		newClients[clientID] = newClient
		h.clients.Store(newClients)
	}
	h.mu.Unlock()

	// 异步清理旧连接
	// 立即关闭旧连接，使其 readPump/writePump 退出，defer 逻辑触发
	if oldClient != nil && !isFromCleanupMap {
		// 先关闭底层连接，关闭后会触发旧 client 的 defer 逻辑
		go oldClient.Close() // 推荐异步关闭，避免阻塞 ServeWS
		log.Printf("Closed old connection for ClientID %s.", clientID)
	}

	// 异步恢复外部订阅 (新连接特有的步骤)
	// 必须异步执行，以避免阻塞 ServeWS 主线程
	if len(subscriptionsToRestore) > 0 {
		go h.restoreSubscriptions(newClient, subscriptionsToRestore)
	}

	defer func() {

		// 清理当前新连接（在连接断开时）
		h.mu.Lock()
		{
			oldClients := h.clients.Load().(map[string]*ClientConn)
			// 只有当要移除的 client 仍然是当前 ClientID 对应的 *ClientConn 时才移除
			if currentClient, exists := oldClients[clientID]; exists && currentClient == newClient {
				newClients := make(map[string]*ClientConn, len(oldClients))
				for k, v := range oldClients {
					if k != clientID { // 按 ClientID 移除
						newClients[k] = v
					}
				}
				h.clients.Store(newClients)
				log.Printf("ClientID %s connection removed from handler.", clientID)
			} else {
				// 如果不相等，说明这个连接已经被一个更新的连接覆盖了，无需从 clients map 中移除
				log.Printf("ClientID %s defer: Connection already replaced, skip map removal.", clientID)
			}
		}
		h.mu.Unlock()

		// 延迟清理逻辑
		// **判断是否已被新连接替换**
		newClient.mu.Lock()
		isReplaced := newClient.replaced // 检查是否是由于重连而断开的
		newClient.mu.Unlock()

		if isReplaced {
			log.Printf("ClientID %s defer: Connection was replaced by a new connection, no cleanup needed.", clientID)
			return
		}

		// 此时，连接是由于超时或客户端主动断开的，但未被替换，需要启动宽限期清理。
		log.Printf("ClientID %s defer: Connection lost. Starting %s cleanup grace period.", clientID, cleanupGrace)

		// 启动宽限期清理
		// 立即从活跃连接 map 中移除后，将其移交给 cleanupMap
		h.cleanupMap.Store(clientID, newClient)

		// 启动一个协程，在宽限期后执行清理
		go func() {
			time.Sleep(cleanupGrace)

			// 1. 检查 cleanupMap 中是否仍存在这个 ClientID
			if conn, loaded := h.cleanupMap.Load(clientID); loaded {
				// 2. 再次检查 conn.replaced 标记 (防止竞态条件)
				clientToCleanup := conn.(*ClientConn)
				clientToCleanup.mu.Lock()
				defer clientToCleanup.mu.Unlock()
				if !clientToCleanup.replaced {
					// 核心修改：循环处理所有通用订阅
					for subKey := range clientToCleanup.Subscriptions {
						// 需要解析 subKey 来确定调用哪个 Unsubscribe
						channel, symbol, period, ok := parseSubKey(subKey)
						if ok && channel == "CANDLE" { // 仅对 K 线执行 Unsubscribe
							h.candleClient.UnsubscribeCandle(context.Background(), symbol, period)
						}
						// TODO: 以后有其他的加入 在这里执行相关业务的取消订阅
					}
				}

				// 3. 无论是清理还是被替换，最终都从 cleanupMap 中移除
				h.cleanupMap.Delete(clientID)
			}
		}()

		// 确保资源关闭
		newClient.Close()
	}()

	// 启动协程
	go newClient.writePump() // 不断从 Send channel 取消息，然后写入 webscoekt
	// 循环读取客户端发来的消息，要求阻塞线程
	// ⚠️这里会阻塞serverWs方法，直到客户端断开连接，断开后会进入defer 清理
	newClient.readPump(h)
}

// 重新订阅
func (g *SubscriptionGateway) restoreSubscriptions(conn *ClientConn, subscribes []string) {
	for _, subKey := range subscribes {
		channel, symbol, period, ok := parseSubKey(subKey)
		if ok {
			g.handleSubscribe(conn, channel, symbol, period, subKey)
		}
	}
}

// 核心：处理客户端的 SUB/UNSUB 请求
func (g *SubscriptionGateway) handleSubscribe(client *ClientConn, channel string, symbol string, period string, subKey string) {
	// 1. 管理客户端本地订阅状态
	client.mu.Lock()
	client.Subscriptions[subKey] = struct{}{}
	client.mu.Unlock()

	// 2. 更新 Gateway 全局过滤映射
	g.addSubscriptionToMap(subKey, client)

	// 3. 调用外部数据源 (根据 channel)
	var err error
	switch channel {
	case "CANDLE":
		// 外部数据源调用
		err = g.candleClient.SubscribeCandle(context.Background(), symbol, period)
	// case "DEPTH":
	//    err = g.depthClient.SubscribeDepth(symbol, period)
	default:
		err = fmt.Errorf("unsupported channel: %s", channel)
	}

	if err != nil {
		log.Printf("Failed to subscribe %s: %v", subKey, err)
		// 回滚客户端状态
		client.mu.Lock()
		delete(client.Subscriptions, subKey)
		client.mu.Unlock()
		g.removeSubscriptionFromMap(subKey, client)
		// TODO: 定向发送错误消息给客户端
	}
}

// SubscriptionGateway.handleUnsubscribe (替换 handleUnsubscribeCandle)
func (g *SubscriptionGateway) handleUnsubscribe(client *ClientConn, subKey string) error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if _, exists := client.Subscriptions[subKey]; !exists {
		return nil
	}

	// 1. 从客户端本地状态中移除
	delete(client.Subscriptions, subKey)

	// 2. 从 Gateway 全局过滤映射中移除
	client.mu.Unlock()
	g.removeSubscriptionFromMap(subKey, client)
	client.mu.Lock() // 重新加锁以保证 defer 释放

	// 3. 调用外部数据源退订 (需要检查是否还有其他订阅者)
	channel, symbol, period, ok := parseSubKey(subKey)
	if !ok {
		return fmt.Errorf("invalid subKey format")
	}

	// 检查是否需要向上游退订
	if g.checkNoActiveSubscribers(subKey) {
		var err error
		switch channel {
		case "CANDLE":
			err = g.candleClient.UnsubscribeCandle(context.Background(), symbol, period)
		// TODO: case "DEPTH":
		default:
			return fmt.Errorf("unsupported channel for unsubscribe: %s", channel)
		}
		if err != nil {
			log.Printf("WARNING: External Unsubscribe failed for %s: %v", subKey, err)
		}
	}
	return nil
}

// 辅助函数：解析通用订阅键 (e.g., "CANDLE:BTC-USDT:15m")
func parseSubKey(key string) (channel, symbol, period string, ok bool) {
	parts := strings.Split(key, ":")
	if len(parts) >= 3 {
		channel = parts[0]
		symbol = parts[1]
		period = parts[2]
		ok = true
	}
	return
}

// 检查全局映射中该 Key 是否还有其他订阅者
func (g *SubscriptionGateway) checkNoActiveSubscribers(subKey string) bool {
	clientsMapInterface, found := g.subscriptionMap.Load(subKey)
	if !found {
		return true // 订阅键不存在，肯定没有活跃订阅者
	}

	clientsMap := clientsMapInterface.(*sync.Map)

	// 检查嵌套 Map 是否包含任何元素
	hasSubscriber := false
	clientsMap.Range(func(key, value interface{}) bool {
		hasSubscriber = true
		return false // 发现一个元素，停止遍历
	})

	// 如果 hasSubscriber 为 true，说明 Map 中还有客户端，返回 false (不是空)
	return !hasSubscriber
}

// addSubscriptionToMap 将 ClientConn 添加到指定的订阅键的订阅者列表中。
func (g *SubscriptionGateway) addSubscriptionToMap(subKey string, client *ClientConn) {

	// 1. 尝试加载或创建该 SubKey 对应的客户端 Map
	// 如果 subKey 不存在，LoadOrStore 会原子地存储传入的 &sync.Map{}
	clientsMapInterface, _ := g.subscriptionMap.LoadOrStore(subKey, &sync.Map{})

	// 断言获取客户端 Map。无论是否是新创建的，clientsMapInterface 都是我们需要的 *sync.Map
	clientsMap := clientsMapInterface.(*sync.Map)

	// 2. 将客户端添加到该 SubKey 的 Map 中
	clientsMap.Store(client.ClientID, client)
}

// removeSubscriptionFromMap 从指定的订阅键的订阅者列表中移除 ClientConn。
func (g *SubscriptionGateway) removeSubscriptionFromMap(subKey string, client *ClientConn) {
	// 1. 查找该 SubKey 对应的客户端 Map
	clientsMapInterface, loaded := g.subscriptionMap.Load(subKey)
	if !loaded {
		// 如果 SubKey 都不存在，无需操作
		return
	}

	clientsMap := clientsMapInterface.(*sync.Map)

	// 2. 从嵌套的 Map 中移除客户端
	clientsMap.Delete(client.ClientID)

	// 3. 优化/清理：检查该 SubKey 的订阅者列表是否为空。
	// 如果为空，则从主 subscriptionMap 中移除该 SubKey，以节省内存。
	// 这一步比较微妙，因为遍历 sync.Map 并计数不是原子的，但为了资源清理，我们仍然执行。

	isEmpty := true
	clientsMap.Range(func(key, value interface{}) bool {
		isEmpty = false
		return false // 发现一个元素，停止遍历
	})

	if isEmpty {
		// 尝试从主 subscriptionMap 中删除这个空的嵌套 Map。
		// 使用 Delete 代替 LoadAndDelete 可以避免在删除时发生写冲突。
		g.subscriptionMap.Delete(subKey)
		log.Printf("Subscription Map: Cleaned up empty SubKey: %s", subKey)
	}
}
