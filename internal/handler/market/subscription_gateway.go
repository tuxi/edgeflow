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
		log.Println("SubscriptionGateway 客户单缺少client_id 拒绝连接.")
		c.Writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	// 升级 websocket
	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println("SubscriptionGateway 升级 websocket 失败:\n", err)
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

	// 1) 先从活跃 clients map 查找旧连接（读锁粒度以内）
	h.mu.Lock()
	{
		currentClients := h.clients.Load().(map[string]*ClientConn)
		if existing, ok := currentClients[clientID]; ok {
			oldClient = existing
			log.Printf("SubscriptionGateway ClientID %s 在clients活跃的已有连接中找到旧连接，准备执行迁移状态.\n", clientID)
		}
	}
	h.mu.Unlock()

	// 如果活跃连接中没有，则检查 cleanupMap (处理宽限期内的重连)
	if oldClient == nil {
		if conn, loaded := h.cleanupMap.Load(clientID); loaded {
			oldClient = conn.(*ClientConn)
			isFromCleanupMap = true
			log.Printf("SubscriptionGateway ClientID %s 在cleanupMap清理map中找到已有连接，准备迁移连接。\n", clientID)
			// 立即从 cleanupMap 中移除，阻止计时器清理
			h.cleanupMap.Delete(clientID)
		}
	}

	// 3) 如果找到了旧连接，准备迁移订阅状态（但不要清空旧的 Subscriptions）
	// 执行状态迁移
	if oldClient != nil {
		log.Printf("SubscriptionGateway ClientID %s 重新连接，开始迁移。\n", clientID)

		// 🚨 锁住旧连接的本地状态，执行迁移
		oldClient.mu.Lock()
		// 复制通用的 Subscriptions
		for subKey := range oldClient.Subscriptions {
			// 复制订阅到新连接的本地缓存，准备恢复订阅
			newClient.Subscriptions[subKey] = struct{}{}
			subscriptionsToRestore = append(subscriptionsToRestore, subKey)
		}

		// 标记旧连接已被替换，阻止其后续宽限期清理做重复的上游 Unsubscribe
		oldClient.replaced = true
		// 注意：不要这里就清空 oldClient.Subscriptions —— 我们保留直到 restore 完成或明确清理
		// 清空旧的通用订阅
		//oldClient.Subscriptions = make(map[string]struct{}, 1) // 不要设置为nil
		oldClient.mu.Unlock()

		log.Printf("SubscriptionGateway ClientID %s: 合并完成 %d 并且将原有订阅迁移到新的连接.", clientID, len(subscriptionsToRestore))
	}

	// 4) 原子 CoW 更新 h.clients（替换/新增）
	h.mu.Lock()
	{

		oldClients := h.clients.Load().(map[string]*ClientConn)
		newClients := make(map[string]*ClientConn, len(oldClients))

		// 复制旧的 map
		for clientId, client := range oldClients {
			newClients[clientId] = client
		}

		// 替换或添加新连接
		newClients[clientID] = newClient
		h.clients.Store(newClients)
	}
	h.mu.Unlock()

	// 5) 立即优雅关闭旧连接（如果存在且不是从 cleanupMap 恢复）
	if oldClient != nil && !isFromCleanupMap {
		// 异步关闭：Close 会触发旧 connection 的 defer 清理逻辑（但 replaced=true 会避免重复 unsubscribe）
		go func(c *ClientConn) {
			c.Close() // 异步关闭，避免阻塞 ServeWS
		}(oldClient)
		log.Printf("SubscriptionGateway 关闭旧的连接 ClientID %s.\n", clientID)
	}

	// 6) 新连接异步恢复订阅（避免阻塞 ServeWS）
	if len(subscriptionsToRestore) > 0 {
		go func(cli *ClientConn, subs []string) {
			// restoreSubscriptions 内部应以 subscriptionMap 为单一真相并做原子化的 upstream subscribe
			h.restoreSubscriptions(cli, subs)
		}(newClient, subscriptionsToRestore)
	}

	// 7) defer 清理：在 readPump 返回（即连接断开）时执行
	defer func() {

		// a) 从 active clients map 中移除（仅当 newClient 仍然是当前映射时）
		// 清理当前新连接（在连接断开时）
		h.mu.Lock()
		{
			currentClients := h.clients.Load().(map[string]*ClientConn)
			// 只有当要移除的 client 仍然是当前 ClientID 对应的 *ClientConn 时才移除
			if cur, exists := currentClients[clientID]; exists && cur == newClient {
				// 构造新的 map （CoW）
				newClients := make(map[string]*ClientConn, len(currentClients))
				for k, v := range currentClients {
					if k == clientID {
						continue
					}
					newClients[k] = v
				}
				h.clients.Store(newClients)
				log.Printf("SubscriptionGateway ClientID %s 已经从活跃的连接中移除连接.\n", clientID)
			} else {
				// 如果不相等，说明这个连接已经被一个更新的连接覆盖了，无需从 clients map 中移除
				log.Printf("SubscriptionGateway ClientID %s defer: 连接已被新连接替换；跳过删除.\n", clientID)
			}
		}
		h.mu.Unlock()

		// b) 如果该连接已被替换（replaced==true），则直接关闭资源并返回（无需宽限期清理）
		newClient.mu.Lock()
		isReplaced := newClient.replaced // 检查是否是由于重连而断开的
		newClient.mu.Unlock()

		if isReplaced {
			log.Printf("SubscriptionGateway ClientID %s defer: 连接已经被替换过，关闭连接并返回,无需宽限期清理\n", clientID)
			return
		}

		// c) 否则进入宽限期清理：先加入 cleanupMap，然后在宽限期后执行最终清理
		// 此时，连接是由于超时或客户端主动断开的，但未被替换，需要启动宽限期清理。
		log.Printf("SubscriptionGateway ClientID %s defer: 连接丢失；进入清理宽限期 (%s)秒 \n", clientID, cleanupGrace)

		// 启动宽限期清理
		// 立即从活跃连接 map 中移除后，将其移交给 cleanupMap
		h.cleanupMap.Store(clientID, newClient)
		go func(id string, clientToCleanup *ClientConn) {
			// 等待宽限期
			time.Sleep(cleanupGrace)

			// 检查 cleanupMap 中记录是否还存在（可能已被新连接恢复）
			if v, loaded := h.cleanupMap.Load(id); loaded {
				candidate := v.(*ClientConn)

				candidate.mu.Lock()
				replacedFlag := candidate.replaced
				candidate.mu.Unlock()

				if !replacedFlag {
					log.Printf("SubscriptionGateway ClientID %s 宽限期已过：正在执行最终清理。\n", id)
					// 关键：调用统一移除函数，从 subscriptionMap 中删除该 client 的所有条目，
					// 并在嵌套 map 变为空时一次性触发上游 Unsubscribe。
					h.removeClientFromAllSubscriptions(id)
				} else {
					log.Printf("SubscriptionGateway ClientID %s 宽限期已过：正在执行最终清理。\n", id)
				}

				// 无论如何都从 cleanupMap 删除该记录
				h.cleanupMap.Delete(id)
			}
			// 最后确保关闭 socket/chan（如果尚未关闭）
			clientToCleanup.Close()

		}(clientID, newClient)
	}()

	// 8) 启动 writePump 和 readPump（writePump 先启动）
	go newClient.writePump() // 不断从 Send channel 取消息，然后写入 webscoekt
	// readPump 会阻塞直到连接关闭（readPump 内部应触发返回，进而执行上面的 defer）
	newClient.readPump(h)
}

// 重新订阅
func (g *SubscriptionGateway) restoreSubscriptions(conn *ClientConn, subscribes []string) {
	for _, subKey := range subscribes {
		g.handleSubscribe(conn, subKey)
	}
}

// 处理客户端的订阅和取消订阅请求
func (g *SubscriptionGateway) handleSubscribe(client *ClientConn, subKey string) {
	// 1. 先把订阅加入主索引（并在必要时向上游 subscribe）
	if err := g.addSubscriptionToMapAndMaybeUpstream(subKey, client); err != nil {
		log.Printf("SubscriptionGateway Failed to subscribe %s: %v", subKey, err)
		return
	}

	// 2. 本地缓存
	client.mu.Lock()
	client.Subscriptions[subKey] = struct{}{}
	client.mu.Unlock()
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

func (g *SubscriptionGateway) addSubscription(subKey string, client *ClientConn) (bool, error) {
	clientsMapInterface, _ := g.subscriptionMap.LoadOrStore(subKey, &sync.Map{})
	clientsMap := clientsMapInterface.(*sync.Map)

	hadSubscribers := false
	clientsMap.Range(func(_, _ interface{}) bool {
		hadSubscribers = true
		return false
	})

	clientsMap.Store(client.ClientID, client)
	return !hadSubscribers, nil
}

// 添加到指定的订阅键的订阅者列表中
func (g *SubscriptionGateway) addSubscriptionToMapAndMaybeUpstream(subKey string, client *ClientConn) error {
	// Load or create nested map
	clientsMapInterface, _ := g.subscriptionMap.LoadOrStore(subKey, &sync.Map{})
	clientsMap := clientsMapInterface.(*sync.Map)

	// 检查是否已有订阅者（先探测）
	hadSubscribers := false
	clientsMap.Range(func(k, v interface{}) bool {
		hadSubscribers = true
		return false
	})

	// 将 client 加入嵌套 map
	clientsMap.Store(client.ClientID, client)

	// 如果之前没有订阅者，则需要向上游订阅一次
	if !hadSubscribers {
		channel, symbol, period, ok := parseSubKey(subKey)
		if !ok {
			// 回滚：从嵌套 map 中删除
			clientsMap.Delete(client.ClientID)
			return fmt.Errorf("invalid subKey %s", subKey)
		}
		switch channel {
		case "CANDLE":
			if err := g.candleClient.SubscribeCandle(context.Background(), symbol, period); err != nil {
				// 回滚
				clientsMap.Delete(client.ClientID)
				return err
			}
		default:
			clientsMap.Delete(client.ClientID)
			return fmt.Errorf("unsupported channel %s", channel)
		}
	}

	return nil
}

// 安全退订（当客户端主动 UN/SUB，或移除 client 时使用）
func (g *SubscriptionGateway) removeSubscriptionFromMapByClientID(subKey string, clientID string) {
	clientsMapInterface, loaded := g.subscriptionMap.Load(subKey)
	if !loaded {
		return
	}
	clientsMap := clientsMapInterface.(*sync.Map)
	clientsMap.Delete(clientID)

	isEmpty := true
	clientsMap.Range(func(_, _ interface{}) bool {
		isEmpty = false
		return false
	})
	if isEmpty {
		g.subscriptionMap.Delete(subKey)
		// 触发上游退订
		channel, symbol, period, ok := parseSubKey(subKey)
		if ok {
			switch channel {
			case "CANDLE":
				if err := g.candleClient.UnsubscribeCandle(context.Background(), symbol, period); err != nil {
					log.Printf("WARNING: SubscriptionGateway External Unsubscribe failed for %s: %v", subKey, err)
				} else {
					log.Printf("SubscriptionGateway Unsubscribed upstream for %s", subKey)
				}
			}
		}
	}
}

// 从 subscriptionMap 中移除 clientID，发现嵌套 map 为空时触发上游 Unsubscribe
func (g *SubscriptionGateway) removeClientFromAllSubscriptions(clientId string) {
	// 遍历所有subKey
	g.subscriptionMap.Range(func(key, val any) bool {
		subKey := key.(string)
		clientsMap := val.(*sync.Map)

		// 从clientsMap 中删除clientId
		clientsMap.Delete(clientId)

		// 检查是否为空
		isEmpty := true
		clientsMap.Range(func(_, _ any) bool {
			isEmpty = false
			return false // 发现一个元素，停止遍历
		})

		if isEmpty {
			// 尝试删除主 map 的条目
			g.subscriptionMap.Delete(subKey)
			// 解析 subKey 并触发上游取消订阅（只触发一次）
			channel, symbol, period, ok := parseSubKey(subKey)
			if ok {
				switch channel {
				case "CANDLE":
					if err := g.candleClient.UnsubscribeCandle(context.Background(), symbol, period); err != nil {
						log.Printf("SubscriptionGateway WARNING: UnsubscribeCandle failed for %s: %v", subKey, err)
					} else {
						log.Printf("SubscriptionGateway Unsubscribed upstream for %s", subKey)
					}
					// TODO: 其他频道
				}
			}
		}

		return true
	})
}
