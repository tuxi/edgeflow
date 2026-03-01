package service

import (
	"context"
	"database/sql"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/kafka"
	pb "edgeflow/pkg/protobuf"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// DefaultSubscriptionRules 定义系统需要自动创建的默认规则
// DefaultSubscriptionRules 定义系统需要自动创建的默认规则
var DefaultSubscriptionRules = []entity.AlertSubscription{

	// --- 1. 系统级极速波动提醒 (RATE) ---
	// 采用更合理的 3.0% 变化作为默认高频提醒的阈值
	{
		UserID:    "SYSTEM_GLOBAL_ALERT",
		InstID:    "BTC-USDT",
		AlertType: 1, // PRICE_ALERT
		Direction: "RATE",

		ChangePercent: sql.NullFloat64{Float64: 3.0, Valid: true}, // 3.0% 变化
		WindowMinutes: sql.NullInt64{Int64: 5, Valid: true},       // 5 分钟窗口

		IsActive: true,
		ID:       "SYS_RATE_BTC_3P_5M", // 更新ID以反映3%
	},

	// --- 2. BTC/USDT 通用价格关口 (千位，步长 1000) ---
	// 监控大整数关口，如 $70000, $71000, $72000...
	{
		UserID:    "SYSTEM_GLOBAL_ALERT",
		InstID:    "BTC-USDT",
		AlertType: 1,
		Direction: "UP",

		// 精度：1.0 (用于浮点数修正)
		BoundaryStep: sql.NullFloat64{Float64: 1.0, Valid: true},
		// 🚀 关口步长 (Magnitude): 1000.0 (触发 $1000$ 的倍数)
		BoundaryMagnitude: sql.NullFloat64{Float64: 1000.0, Valid: true},

		IsActive: true,
		ID:       "SYS_BOUND_BTC_UP_1K",
	},
	{
		UserID:            "SYSTEM_GLOBAL_ALERT",
		InstID:            "BTC-USDT",
		AlertType:         1,
		Direction:         "DOWN",
		BoundaryStep:      sql.NullFloat64{Float64: 1.0, Valid: true},
		BoundaryMagnitude: sql.NullFloat64{Float64: 1000.0, Valid: true},
		IsActive:          true,
		ID:                "SYS_BOUND_BTC_DOWN_1K",
	},

	// --- 3. ETH/USDT 通用价格关口 (百位，步长 100) ---
	// 监控 $2700, $2800, $2900...
	{
		UserID:       "SYSTEM_GLOBAL_ALERT",
		InstID:       "ETH-USDT",
		AlertType:    1,
		Direction:    "UP",
		BoundaryStep: sql.NullFloat64{Float64: 1.0, Valid: true},
		// 🚀 关口步长 (Magnitude): 100.0 (触发 $100$ 的倍数)
		BoundaryMagnitude: sql.NullFloat64{Float64: 100.0, Valid: true},
		IsActive:          true,
		ID:                "SYS_BOUND_ETH_UP_100",
	},
	{
		UserID:            "SYSTEM_GLOBAL_ALERT",
		InstID:            "ETH-USDT",
		AlertType:         1,
		Direction:         "DOWN",
		BoundaryStep:      sql.NullFloat64{Float64: 1.0, Valid: true},
		BoundaryMagnitude: sql.NullFloat64{Float64: 100.0, Valid: true},
		IsActive:          true,
		ID:                "SYS_BOUND_ETH_DOWN_100",
	},

	// --- 4. SOL/USDT 通用价格关口 (五刀，步长 5.0) ---
	// 监控 $125, $130, $135...
	{
		UserID:       "SYSTEM_GLOBAL_ALERT",
		InstID:       "SOL-USDT",
		AlertType:    1,
		Direction:    "UP",
		BoundaryStep: sql.NullFloat64{Float64: 0.1, Valid: true}, // 小数点后一位精度修正
		// 🚀 关口步长 (Magnitude): 5.0
		BoundaryMagnitude: sql.NullFloat64{Float64: 5.0, Valid: true},
		IsActive:          true,
		ID:                "SYS_BOUND_SOL_UP_5",
	},
	{
		UserID:            "SYSTEM_GLOBAL_ALERT",
		InstID:            "SOL-USDT",
		AlertType:         1,
		Direction:         "DOWN",
		BoundaryStep:      sql.NullFloat64{Float64: 0.1, Valid: true},
		BoundaryMagnitude: sql.NullFloat64{Float64: 5.0, Valid: true},
		IsActive:          true,
		ID:                "SYS_BOUND_SOL_DOWN_5",
	},

	// --- 5. DOGE/USDT 通用价格关口 (分位，步长 0.01) ---
	// 监控 $0.13, $0.14, $0.15...
	{
		UserID:       "SYSTEM_GLOBAL_ALERT",
		InstID:       "DOGE-USDT",
		AlertType:    1,
		Direction:    "UP",
		BoundaryStep: sql.NullFloat64{Float64: 0.0001, Valid: true}, // 小数点后四位精度修正
		// 🚀 关口步长 (Magnitude): 0.01
		BoundaryMagnitude: sql.NullFloat64{Float64: 0.01, Valid: true},
		IsActive:          true,
		ID:                "SYS_BOUND_DOGE_UP_001",
	},
	{
		UserID:            "SYSTEM_GLOBAL_ALERT",
		InstID:            "DOGE-USDT",
		AlertType:         1,
		Direction:         "DOWN",
		BoundaryStep:      sql.NullFloat64{Float64: 0.0001, Valid: true},
		BoundaryMagnitude: sql.NullFloat64{Float64: 0.01, Valid: true},
		IsActive:          true,
		ID:                "SYS_BOUND_DOGE_DOWN_001",
	},
}

// AlertService 用于消费上游告警来源并提供订阅通道给 gateway。
type AlertService struct {
	producer kafka.ProducerService
	dao      dao.AlertDAO
	// 价格提醒订阅存储 (InstID -> []Subscription)
	// ⚠️ 注意：这是一个临界资源，必须在 mu 锁保护下访问
	priceAlerts map[string][]*PriceAlertSubscription
	mu          sync.RWMutex
}

type AlertPublisher interface {
	Publish(msg *pb.AlertMessage)
	GetSubscriptionsForInstID(instID string) []*PriceAlertSubscription
	// 处理订阅触发：更新时间、价格，并根据标志决定是否停用
	HandleAlertTrigger(instID string, subscriptionID string, triggeredPrice float64, shouldDeactivate bool)
	// 标记为已重置，重新激活
	MarkSubscriptionAsReset(instID string, subscriptionID string)
}

// 提醒订阅结构体（MDS 内部存储）
type PriceAlertSubscription struct {
	UserID         string // 对应 Kafka Key 和客户端 ID
	SubscriptionID string // 用户的订阅唯一 ID
	InstID         string // 交易对，如 BTC-USDT
	IsActive       bool   // 是否已触发或活跃

	// 极速提醒字段
	ChangePercent float64 // 变化百分比 (例如 5.0 代表 5%)
	WindowMinutes int     // 时间窗口 (例如 5 代表 5分钟)

	// 现有价格突破字段
	TargetPrice float64 // 目标价格
	Direction   string  // "UP", "DOWN" (现在也用于极速提醒的上升/下降)

	LastTriggeredPrice float64    // 上次触发时的价格（用于判断是否重置）
	LastTriggeredTime  *time.Time // 记录上次触发提醒的时间

	BoundaryStep      float64 // 0.01 表示以 0.01 为单位跨越
	BoundaryMagnitude float64
}

func NewAlertService(producer kafka.ProducerService, dao dao.AlertDAO) *AlertService {
	s := &AlertService{
		producer:    producer,
		dao:         dao,
		priceAlerts: make(map[string][]*PriceAlertSubscription),
	}
	// 🚀 启动时从数据库加载所有活跃订阅到内存
	s.loadActiveSubscriptions()
	s.createDefaultSubscriptions()
	return s
}

// createDefaultSubscriptions 检查数据库中是否已存在系统默认订阅，若无则创建
func (s *AlertService) createDefaultSubscriptions() {
	log.Println("INFO: 正在检查并创建系统默认订阅...")

	for _, rule := range DefaultSubscriptionRules {
		// 确保使用 rule 变量的副本，防止在循环中被修改
		currentRule := rule

		ruleID := currentRule.ID // 直接使用定义好的固定 ID

		// 2. 检查数据库中是否已存在此 ID
		// 🚨 需要 AlertDAO 增加 GetSubscriptionByID 方法
		existingSub, err := s.dao.GetSubscriptionByID(context.Background(), ruleID)

		if err == nil && existingSub.ID == ruleID {
			// 订阅已存在，跳过创建
			log.Printf("INFO: 系统默认订阅 %s 已存在，跳过。", ruleID)

			// 确保内存中也有该订阅（如果 loadActiveSubscriptions 没有加载到）
			s.AddSubscriptionToMemory(&existingSub)

			continue
		}

		// 3. 数据库中不存在，则创建
		currentRule.CreatedAt = time.Now()
		currentRule.UpdatedAt = time.Now()

		if err := s.dao.CreateSubscription(context.Background(), &currentRule); err != nil {
			log.Printf("ERROR: 创建系统默认订阅 %s 失败: %v", ruleID, err)
			continue
		}

		// 4. 🚀 创建成功，同步更新内存
		s.AddSubscriptionToMemory(&currentRule)

		log.Printf("INFO: 成功创建系统默认订阅 %s。", ruleID)
	}
}

// loadActiveSubscriptions 从 DB 加载活跃订阅到内存
func (s *AlertService) loadActiveSubscriptions() {
	dbSubs, err := s.dao.GetAllActiveSubscriptions(context.Background())
	if err != nil {
		log.Fatalf("FATAL: AlertService 启动时无法加载活跃订阅: %v", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 清空旧数据
	s.priceAlerts = make(map[string][]*PriceAlertSubscription)

	for _, dbSub := range dbSubs {
		sub := &PriceAlertSubscription{
			SubscriptionID:     dbSub.ID,
			UserID:             dbSub.UserID,
			InstID:             dbSub.InstID,
			IsActive:           dbSub.IsActive,
			ChangePercent:      dbSub.ChangePercent.Float64,
			WindowMinutes:      int(dbSub.WindowMinutes.Int64),
			TargetPrice:        dbSub.TargetPrice.Float64,
			Direction:          dbSub.Direction,
			LastTriggeredPrice: dbSub.LastTriggeredPrice.Float64,
			LastTriggeredTime:  dbSub.LastTriggeredTime,
			BoundaryStep:       dbSub.BoundaryStep.Float64,
			BoundaryMagnitude:  dbSub.BoundaryMagnitude.Float64,
		}
		s.priceAlerts[sub.InstID] = append(s.priceAlerts[sub.InstID], sub)
	}
	log.Printf("AlertService 成功加载 %d 个活跃订阅。", len(dbSubs))
}

func (s *AlertService) Publish(msg *pb.AlertMessage) {
	if msg == nil {
		return
	}
	if msg.UserId == "SYSTEM_GLOBAL_ALERT" {
		s.PublishBroadcast(msg)
	} else {
		s.PublishToDevice(msg)
	}
}

// 写入全量推送 Topic
func (s *AlertService) PublishBroadcast(msg *pb.AlertMessage) {
	extra := msg.GetExtra()
	extraBytes, err := json.Marshal(extra)
	if err != nil {
		return
	}

	// 保存历史记录 (同步或异步取决于业务对丢历史记录的容忍度)
	history := &entity.AlertHistory{
		ID:             msg.GetId(),
		UserID:         msg.UserId,
		SubscriptionID: msg.GetSubscriptionId(),
		Title:          msg.GetTitle(),
		Content:        msg.GetContent(),
		Level:          int(msg.GetLevel()),
		AlertType:      int(msg.GetAlertType()),
		Timestamp:      msg.GetTimestamp(),
		ExtraJSON:      string(extraBytes),
	}
	if err := s.dao.SaveAlertHistory(context.Background(), history); err != nil {
		log.Printf("WARN: 保存提醒历史失败 ID=%s: %v", history.ID, err)
		// 允许失败，继续推送 Kafka
	}

	protoMsg := kafka.Message{
		Key: "ALERT_BROADCAST", // 固定KEY
		Data: &pb.WebSocketMessage{
			Type:    "ALERT_SUBSCRIBE",
			Payload: &pb.WebSocketMessage_AlertMessage{AlertMessage: msg},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	if err := s.producer.Produce(ctx, kafka.TopicAlertSystem, protoMsg); err != nil {
		log.Printf("ERROR: AlertService topic=%s 广播写入kafka数据失败: %v", kafka.TopicAlertSystem, err)
	}
}

// 写入定向推送 Topic
func (s *AlertService) PublishToDevice(msg *pb.AlertMessage) {

	extra := msg.GetExtra()
	extraBytes, err := json.Marshal(extra)
	if err != nil {
		return
	}

	// 保存历史记录 (同步或异步取决于业务对丢历史记录的容忍度)
	history := &entity.AlertHistory{
		ID:             msg.GetId(),
		UserID:         msg.UserId,
		SubscriptionID: msg.GetSubscriptionId(),
		Title:          msg.GetTitle(),
		Content:        msg.GetContent(),
		Level:          int(msg.GetLevel()),
		AlertType:      int(msg.GetAlertType()),
		Timestamp:      msg.GetTimestamp(),
		ExtraJSON:      string(extraBytes),
	}
	if err := s.dao.SaveAlertHistory(context.Background(), history); err != nil {
		log.Printf("WARN: 保存提醒历史失败 ID=%s: %v", history.ID, err)
		// 允许失败，继续推送 Kafka
	}

	// 1. 构造消息
	protoMsg := kafka.Message{
		// Kafka Key 必须是 deviceId
		Key: msg.UserId,
		Data: &pb.WebSocketMessage{
			Type:    "ALERT_DIRECT",
			Payload: &pb.WebSocketMessage_AlertMessage{AlertMessage: msg},
		},
	}

	// 2. 写入定向 Topic
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()

	// 使用定向推送 Topic
	if err := s.producer.Produce(ctx, kafka.TopicAlertDirect, protoMsg); err != nil {
		// 定向推送写入失败，记录日志
		log.Printf("ERROR: AlertService 定向推送写入 Kafka失败 (Device: %s): %v", msg.UserId, err)
	}
}

// AlertService 暴露获取订阅的方法
// MDS 将通过这个方法获取订阅列表
func (s *AlertService) GetSubscriptionsForInstID(instID string) []*PriceAlertSubscription {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 返回副本或不可修改视图是最佳实践
	subs, ok := s.priceAlerts[instID]
	if !ok {
		return nil
	}
	// 返回副本，防止外部修改
	return append([]*PriceAlertSubscription{}, subs...)
}

// AlertService 管理订阅的方法 (供外部 API 调用)
func (s *AlertService) AddPriceAlert(sub PriceAlertSubscription) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list := s.priceAlerts[sub.InstID]
	// 假设这里执行去重、更新等复杂逻辑
	list = append(list, &sub)
	s.priceAlerts[sub.InstID] = list
}

// MarkSubscriptionAsTriggered 标记订阅为已触发，并记录价格
func (s *AlertService) HandleAlertTrigger(instID string, subscriptionID string, triggeredPrice float64, shouldDeactivate bool) {
	// 更新内存状态 (用于后续 Ticker 立即生效)
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, ok := s.priceAlerts[instID]
	if !ok {
		log.Printf("WARN: AlertService 尝试标记触发，但 InstID %s 不存在。", instID)
		return
	}

	now := time.Now()

	for _, sub := range subs {
		if sub.SubscriptionID == subscriptionID {
			// 1. 统一更新触发信息
			sub.LastTriggeredPrice = triggeredPrice
			sub.LastTriggeredTime = &now // 🚀 总是更新时间

			isActive := true

			// 2. 根据参数决定是否停用
			if shouldDeactivate {
				sub.IsActive = false
				isActive = false
				log.Printf("INFO: 订阅 %s 触发并停用 (价格: %.2f)", subscriptionID, triggeredPrice)
			} else {
				// 对于通用关口，保持 IsActive = true
				log.Printf("INFO: 订阅 %s 触发并更新时间 (价格: %.2f)", subscriptionID, triggeredPrice)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()

			// 异步持久化到 DB (传入所有必要参数)
			go func(id string, active bool, price float64, t *time.Time, ctx context.Context) {
				if t == nil {
					t = &now
				}
				// 传入当前时间 t，DAO 会更新 last_triggered_time
				if err := s.dao.UpdateSubscriptionAfterTrigger(ctx, id, active, price, *t); err != nil {
					log.Printf("ERROR: DAO 触发订阅失败 ID=%s: %v", id, err)
				}
			}(sub.SubscriptionID, isActive, sub.LastTriggeredPrice, sub.LastTriggeredTime, ctx)

			return
		}
	}
}

// MarkSubscriptionAsReset 标记订阅为已重置，重新激活
func (s *AlertService) MarkSubscriptionAsReset(instID string, subscriptionID string) {
	// 更新内存状态 (用于后续 Ticker 立即生效)
	s.mu.Lock()
	defer s.mu.Unlock()

	subs, ok := s.priceAlerts[instID]
	if !ok {
		log.Printf("WARN: AlertService 尝试标记重置，但 InstID %s 不存在。", instID)
		return
	}

	for _, sub := range subs {
		// 只有 IsActive = false 的订阅才需要重置
		if sub.SubscriptionID == subscriptionID && !sub.IsActive {
			sub.IsActive = true
			sub.LastTriggeredPrice = 0 // 清除触发价格，准备下一次检测
			// 注意：我们不清除 LastTriggeredTime，保留它作为“上一次（哪怕是已重置的）触发时间”供参考

			log.Printf("INFO: 订阅 %s 已重置 (重新激活)。", subscriptionID)

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
			defer cancel()

			// 复用 UpdateSubscriptionAfterTrigger 持久化到 DB
			// 参数说明：
			// isActive: true (激活)
			// price: 0 (清除价格)
			// time: time.Time{} (传零值，表示不更新触发时间，保留原有的历史时间)
			go func(id string, ctx context.Context) {
				// 传入零值时间，利用 DAO 层的判断逻辑跳过更新时间字段
				if err := s.dao.UpdateSubscriptionAfterTrigger(ctx, id, true, 0, time.Time{}); err != nil {
					log.Printf("ERROR: DAO 重置订阅失败 ID=%s: %v", id, err)
				}
			}(sub.SubscriptionID, ctx)

			log.Printf("INFO: 订阅 %s 已标记为已重置 (重新激活)。", subscriptionID)
			return
		}
	}
}

// CreateSubscription 处理 POST /api/v1/alerts/subscriptions
func (g *AlertService) CreateSubscription(ctx context.Context, req model.CreateUpdateSubscriptionRequest) error {

	// 1. 构造 model.AlertSubscription 对象 (需要处理 float64 到 sql.NullFloat64 的转换)
	sub := g.mapRequestToModel(&req)
	sub.ID = uuid.NewString() // 生成新的 ID
	sub.IsActive = true       // 新订阅默认为活跃状态

	// 2. 调用 AlertDAO 写入数据库
	if err := g.dao.CreateSubscription(ctx, sub); err != nil {
		return err
	}

	// 更新内存 (必须同步更新内存，才能立即开始接收提醒)
	g.AddSubscriptionToMemory(sub) // AlertService 需要增加这个方法

	return nil
}

// GetSubscriptions 处理 GET /api/v1/alerts/subscriptions
func (g *AlertService) GetSubscriptionsByUserID(ctx context.Context, userID string) ([]model.SubscriptionResponse, error) {

	// 1. 调用 DAO 获取用户所有订阅
	// 假设 AlertDAO 中新增 GetSubscriptionsByUserID 方法
	dbSubs, err := g.dao.GetSubscriptionsByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	// 2. 构造响应列表 (需要将 model.AlertSubscription 转换为 SubscriptionResponse)
	response := g.mapModelsToResponse(dbSubs)

	return response, nil
}

// UpdateSubscription 处理 PUT /api/v1/alerts/subscriptions/{id}
func (s *AlertService) UpdateSubscription(ctx context.Context, subID string, req model.CreateUpdateSubscriptionRequest) error {

	// 1. 构造 model.AlertSubscription 对象 (需要从 DB 加载旧记录以获取 CreatedAt/状态等，这里简化)
	sub := s.mapRequestToModel(&req)
	sub.ID = subID // 设置 ID
	// ⚠️ 复杂逻辑：需要从 DB 查出旧记录，保留 IsActive 状态、LastTriggeredPrice 等，再应用新规则。
	// 这里简化为直接调用 UpdateSubscription，假设只更新规则字段。

	// 2. 更新数据库
	if err := s.dao.UpdateSubscription(ctx, sub); err != nil {
		return err
	}

	// 3. 🚀 同步更新 AlertService 内存
	s.AddSubscriptionToMemory(sub)

	return nil
}

// DeleteSubscription 处理 DELETE /api/v1/alerts/subscriptions/{id}
func (g *AlertService) DeleteSubscription(ctx context.Context, subID string, instID string) error {

	// 1. 调用 DAO 删除数据库记录
	if err := g.dao.DeleteSubscription(ctx, subID); err != nil {
		return err
	}

	// 从内存中移除该订阅
	g.RemoveSubscriptionFromMemory(subID, instID)

	return nil
}

// mapRequestToModel 将 API 请求结构体转换为数据库 Model 结构体
func (s *AlertService) mapRequestToModel(req *model.CreateUpdateSubscriptionRequest) *entity.AlertSubscription {
	sub := &entity.AlertSubscription{
		UserID:    req.UserID,
		InstID:    req.InstID,
		AlertType: req.AlertType, // 假设 AlertType 是 int32
		Direction: req.Direction,
		// 其他字段在创建和更新时通常不需要设置，如 CreatedAt, UpdatedAt
	}

	// 价格突破字段转换 (如果 TargetPrice > 0，则设置值)
	if req.TargetPrice > 0 {
		sub.TargetPrice = sql.NullFloat64{Float64: req.TargetPrice, Valid: true}
	} else {
		sub.TargetPrice = sql.NullFloat64{Valid: false}
	}

	// 极速提醒字段转换
	if req.ChangePercent > 0 {
		sub.ChangePercent = sql.NullFloat64{Float64: req.ChangePercent, Valid: true}
	} else {
		sub.ChangePercent = sql.NullFloat64{Valid: false}
	}

	if req.WindowMinutes > 0 {
		sub.WindowMinutes = sql.NullInt64{Int64: int64(req.WindowMinutes), Valid: true}
	} else {
		sub.WindowMinutes = sql.NullInt64{Valid: false}
	}

	// 如果是创建操作，这些字段由 DB 或 AlertService 处理
	// 如果是更新操作，需要确保这些字段也被正确处理，通常需要从 DB 先加载旧记录。

	// 默认值/状态处理：
	sub.CreatedAt = time.Now() // 仅在创建时使用，更新时会被覆盖
	sub.UpdatedAt = time.Now()

	return sub
}

// mapModelsToResponse 将数据库模型切片转换为 API 响应切片
func (g *AlertService) mapModelsToResponse(dbSubs []entity.AlertSubscription) []model.SubscriptionResponse {
	if len(dbSubs) == 0 {
		return []model.SubscriptionResponse{}
	}

	responseList := make([]model.SubscriptionResponse, len(dbSubs))

	for i, dbSub := range dbSubs {
		// 转换逻辑：直接使用 Float64/Int64 字段，Go 会自动处理。
		// 如果 Valid=false，Float64/Int64 返回零值 (0.0 或 0)，这符合 API 响应的期望。
		responseList[i] = model.SubscriptionResponse{
			ID:        dbSub.ID,
			UserID:    dbSub.UserID,
			InstID:    dbSub.InstID,
			AlertType: int(dbSub.AlertType),
			Direction: dbSub.Direction,

			// 🚀 核心：安全转换 Nullable 字段
			TargetPrice:   dbSub.TargetPrice.Float64,
			ChangePercent: dbSub.ChangePercent.Float64,
			WindowMinutes: int(dbSub.WindowMinutes.Int64),

			IsActive:           dbSub.IsActive,
			LastTriggeredPrice: dbSub.LastTriggeredPrice.Float64,
		}
	}
	return responseList
}

// AddSubscriptionToMemory 供 Gateway 调用，用于在内存中添加或更新订阅
func (s *AlertService) AddSubscriptionToMemory(dbSub *entity.AlertSubscription) {
	// 1. 转换为 Service 内部结构
	sub := mapModelToServiceSubscription(dbSub)

	s.mu.Lock()
	defer s.mu.Unlock()

	instID := sub.InstID

	// 检查 InstID 列表是否存在，如果不存在则创建
	if _, ok := s.priceAlerts[instID]; !ok {
		s.priceAlerts[instID] = make([]*PriceAlertSubscription, 0)
	}

	list := s.priceAlerts[instID]
	found := false

	// 查找是否已存在（即 PUT 更新操作）
	for i, existingSub := range list {
		if existingSub.SubscriptionID == sub.SubscriptionID {
			list[i] = sub // 🚀 替换旧的订阅对象
			s.priceAlerts[instID] = list
			found = true
			break
		}
	}

	// 如果是新添加 (POST)，则追加
	if !found {
		s.priceAlerts[instID] = append(list, sub)
	}
	log.Printf("INFO: 内存中订阅 %s (InstID: %s) 已更新/添加。", sub.SubscriptionID, instID)
}

// RemoveSubscriptionFromMemory 供 Gateway 调用，从内存中移除订阅
// 传入 instID 是为了快速定位 map key，避免遍历整个 map
func (s *AlertService) RemoveSubscriptionFromMemory(subscriptionID string, instID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	list, ok := s.priceAlerts[instID]
	if !ok {
		log.Printf("WARN: 尝试移除订阅 %s，但 InstID %s 列表不存在。", subscriptionID, instID)
		return
	}

	// 遍历列表，找到匹配的 ID 并移除
	for i, sub := range list {
		if sub.SubscriptionID == subscriptionID {
			// 使用切片技巧移除元素
			s.priceAlerts[instID] = append(list[:i], list[i+1:]...)

			// 如果移除后列表为空，清理 map entry
			if len(s.priceAlerts[instID]) == 0 {
				delete(s.priceAlerts, instID)
			}

			log.Printf("INFO: 内存中订阅 %s (InstID: %s) 已移除。", subscriptionID, instID)
			return
		}
	}
	log.Printf("WARN: 尝试移除订阅 %s，但在 InstID %s 列表中未找到。", subscriptionID, instID)
}

// mapModelToServiceSubscription 将数据库 model.AlertSubscription
// 转换为 service.PriceAlertSubscription 内存结构
func mapModelToServiceSubscription(dbSub *entity.AlertSubscription) *PriceAlertSubscription {
	sub := &PriceAlertSubscription{
		SubscriptionID: dbSub.ID,
		UserID:         dbSub.UserID,
		InstID:         dbSub.InstID,

		// 基础字段
		// 假设 AlertType 字段在 model 中为 int32 或 int，需要保持一致
		// AlertType:         int(dbSub.AlertType),
		Direction: dbSub.Direction,
		IsActive:  dbSub.IsActive,

		// 价格突破字段
		TargetPrice: dbSub.TargetPrice.Float64,

		// 极速提醒字段
		ChangePercent: dbSub.ChangePercent.Float64,
		WindowMinutes: int(dbSub.WindowMinutes.Int64),

		// 状态字段
		LastTriggeredPrice: dbSub.LastTriggeredPrice.Float64,
		LastTriggeredTime:  dbSub.LastTriggeredTime,
		BoundaryStep:       dbSub.BoundaryStep.Float64,
		BoundaryMagnitude:  dbSub.BoundaryMagnitude.Float64,
	}

	return sub
}

func (s *AlertService) GetAllHistoriesByID(ctx context.Context, userId string, alertType int, limit, offset int) ([]entity.AlertHistory, error) {
	if limit == 0 {
		limit = 100
	}
	// 先查找系统全局提醒
	globalAlerts, err := s.dao.GetHistoryByUserID(ctx, "SYSTEM_GLOBAL_ALERT", alertType, limit, offset)
	if err != nil {
		return nil, err
	}

	// 再查找当前用户的提醒
	histories, err := s.dao.GetHistoryByUserID(ctx, userId, alertType, limit, offset)
	// 合并数据
	if err != nil {
		return nil, err
	}
	allAlerts := append(globalAlerts, histories...)
	return allAlerts, nil
}
