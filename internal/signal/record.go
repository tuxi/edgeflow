package signal

import (
	"sync"
	"time"
)

// TradeRecord 单次交易记录
type TradeRecord struct {
	Action    Action    `json:"action"`    // 交易动作
	Symbol    string    `json:"symbol"`    // 交易对
	Timestamp time.Time `json:"timestamp"` // 交易时间
	Price     float64   `json:"price"`     // 交易价格
	Side      string    `json:"side"`      // 方向 (LONG/SHORT)
}

// SymbolStats 单个交易对的统计信息
type SymbolStats struct {
	Symbol           string        `json:"symbol"`            // 交易对
	ConsecutiveOpens int           `json:"consecutive_opens"` // 连续开仓次数
	ConsecutiveAdds  int           `json:"consecutive_adds"`  // 连续加仓次数
	LastOpenTime     time.Time     `json:"last_open_time"`    // 最后开仓时间
	LastAddTime      time.Time     `json:"last_add_time"`     // 最后加仓时间
	TradeHistory     []TradeRecord `json:"trade_history"`     // 交易历史（最近N笔）
	LastResetTime    time.Time     `json:"last_reset_time"`   // 上次重置时间
	TotalOpensToday  int           `json:"total_opens_today"` // 今日总开仓次数
	TotalAddsToday   int           `json:"total_adds_today"`  // 今日总加仓次数
	mu               sync.RWMutex  `json:"-"`                 // 读写锁
}

// TradeLimiterConfig 配置参数
type TradeLimiterConfig struct {
	MaxConsecutiveOpens int           `json:"max_consecutive_opens"` // 最大连续开仓次数
	MaxConsecutiveAdds  int           `json:"max_consecutive_adds"`  // 最大连续加仓次数
	OpenCooldownPeriod  time.Duration `json:"open_cooldown_period"`  // 开仓冷却时间
	AddCooldownPeriod   time.Duration `json:"add_cooldown_period"`   // 加仓冷却时间
	MaxOpensPerDay      int           `json:"max_opens_per_day"`     // 每日最大开仓次数
	MaxAddsPerDay       int           `json:"max_adds_per_day"`      // 每日最大加仓次数
	HistoryKeepCount    int           `json:"history_keep_count"`    // 保留历史记录数量
	ResetHour           int           `json:"reset_hour"`            // 每日重置时间（小时）
}

// TradeLimiter 交易频次控制器
type TradeLimiter struct {
	config      TradeLimiterConfig      `json:"config"`
	symbolStats map[string]*SymbolStats `json:"symbol_stats"`
	mu          sync.RWMutex            `json:"-"`
}

// NewTradeLimiter 创建交易频次控制器
func NewTradeLimiter(config TradeLimiterConfig) *TradeLimiter {
	// 设置默认配置
	if config.MaxConsecutiveOpens == 0 {
		config.MaxConsecutiveOpens = 3 // 最多连续开仓3次
	}
	if config.MaxConsecutiveAdds == 0 {
		config.MaxConsecutiveAdds = 2 // 最多连续加仓2次
	}
	if config.OpenCooldownPeriod == 0 {
		config.OpenCooldownPeriod = 30 * time.Minute // 开仓冷却30分钟
	}
	if config.AddCooldownPeriod == 0 {
		config.AddCooldownPeriod = 15 * time.Minute // 加仓冷却15分钟
	}
	if config.MaxOpensPerDay == 0 {
		config.MaxOpensPerDay = 10 // 每日最多开仓10次
	}
	if config.MaxAddsPerDay == 0 {
		config.MaxAddsPerDay = 15 // 每日最多加仓15次
	}
	if config.HistoryKeepCount == 0 {
		config.HistoryKeepCount = 20 // 保留最近20笔交易记录
	}
	if config.ResetHour == 0 {
		config.ResetHour = 0 // 凌晨0点重置
	}

	return &TradeLimiter{
		config:      config,
		symbolStats: make(map[string]*SymbolStats),
	}
}

// CanOpen 检查是否可以开仓
func (tl *TradeLimiter) CanOpen(symbol string) bool {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	stats := tl.getOrCreateSymbolStats(symbol)
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	tl.checkAndResetDaily(stats)

	now := time.Now()

	// 检查连续开仓次数
	if stats.ConsecutiveOpens >= tl.config.MaxConsecutiveOpens {
		return false
	}

	// 检查开仓冷却时间
	if !stats.LastOpenTime.IsZero() && now.Sub(stats.LastOpenTime) < tl.config.OpenCooldownPeriod {
		return false
	}

	// 检查每日开仓次数限制
	if stats.TotalOpensToday >= tl.config.MaxOpensPerDay {
		return false
	}

	return true
}

// CanAdd 检查是否可以加仓
func (tl *TradeLimiter) CanAdd(symbol string) bool {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	stats := tl.getOrCreateSymbolStats(symbol)
	stats.mu.RLock()
	defer stats.mu.RUnlock()

	tl.checkAndResetDaily(stats)

	now := time.Now()

	// 检查连续加仓次数
	if stats.ConsecutiveAdds >= tl.config.MaxConsecutiveAdds {
		return false
	}

	// 检查加仓冷却时间
	if !stats.LastAddTime.IsZero() && now.Sub(stats.LastAddTime) < tl.config.AddCooldownPeriod {
		return false
	}

	// 检查每日加仓次数限制
	if stats.TotalAddsToday >= tl.config.MaxAddsPerDay {
		return false
	}

	return true
}

// RecordTrade 记录交易
func (tl *TradeLimiter) RecordTrade(symbol string, action Action, price float64, side string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	stats := tl.getOrCreateSymbolStats(symbol)
	stats.mu.Lock()
	defer stats.mu.Unlock()

	now := time.Now()
	record := TradeRecord{
		Action:    action,
		Symbol:    symbol,
		Timestamp: now,
		Price:     price,
		Side:      side,
	}

	// 添加到历史记录
	stats.TradeHistory = append(stats.TradeHistory, record)
	if len(stats.TradeHistory) > tl.config.HistoryKeepCount {
		stats.TradeHistory = stats.TradeHistory[1:] // 移除最旧的记录
	}

	// 更新统计信息
	switch action {
	case ActOpen:
		stats.ConsecutiveOpens++
		stats.ConsecutiveAdds = 0 // 开仓后重置加仓计数
		stats.LastOpenTime = now
		stats.TotalOpensToday++
	case ActAdd:
		stats.ConsecutiveAdds++
		stats.ConsecutiveOpens = 0 // 加仓后重置开仓计数
		stats.LastAddTime = now
		stats.TotalAddsToday++
	}
}

// RecordClose 记录平仓（重置连续计数）
func (tl *TradeLimiter) RecordClose(symbol string, price float64, side string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	stats := tl.getOrCreateSymbolStats(symbol)
	stats.mu.Lock()
	defer stats.mu.Unlock()

	now := time.Now()
	record := TradeRecord{
		Action:    ActClose,
		Symbol:    symbol,
		Timestamp: now,
		Price:     price,
		Side:      side,
	}

	// 添加到历史记录
	stats.TradeHistory = append(stats.TradeHistory, record)
	if len(stats.TradeHistory) > tl.config.HistoryKeepCount {
		stats.TradeHistory = stats.TradeHistory[1:]
	}

	// 平仓后重置所有连续计数
	stats.ConsecutiveOpens = 0
	stats.ConsecutiveAdds = 0
}

// GetSymbolStats 获取交易对统计信息
func (tl *TradeLimiter) GetSymbolStats(symbol string) *SymbolStats {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	if stats, exists := tl.symbolStats[symbol]; exists {
		stats.mu.RLock()
		defer stats.mu.RUnlock()

		// 返回副本，避免外部修改
		statsCopy := *stats
		statsCopy.TradeHistory = make([]TradeRecord, len(stats.TradeHistory))
		copy(statsCopy.TradeHistory, stats.TradeHistory)
		return &statsCopy
	}
	return nil
}

// GetAllStats 获取所有交易对统计信息
func (tl *TradeLimiter) GetAllStats() map[string]*SymbolStats {
	tl.mu.RLock()
	defer tl.mu.RUnlock()

	result := make(map[string]*SymbolStats)
	for symbol, stats := range tl.symbolStats {
		stats.mu.RLock()
		statsCopy := *stats
		statsCopy.TradeHistory = make([]TradeRecord, len(stats.TradeHistory))
		copy(statsCopy.TradeHistory, stats.TradeHistory)
		result[symbol] = &statsCopy
		stats.mu.RUnlock()
	}
	return result
}

// ResetSymbol 重置指定交易对的统计信息
func (tl *TradeLimiter) ResetSymbol(symbol string) {
	tl.mu.Lock()
	defer tl.mu.Unlock()

	if stats, exists := tl.symbolStats[symbol]; exists {
		stats.mu.Lock()
		defer stats.mu.Unlock()

		stats.ConsecutiveOpens = 0
		stats.ConsecutiveAdds = 0
		stats.TotalOpensToday = 0
		stats.TotalAddsToday = 0
		stats.LastResetTime = time.Now()
	}
}

// 获取或创建交易对统计信息（内部方法，调用前需要加锁）
func (tl *TradeLimiter) getOrCreateSymbolStats(symbol string) *SymbolStats {
	if stats, exists := tl.symbolStats[symbol]; exists {
		return stats
	}

	stats := &SymbolStats{
		Symbol:           symbol,
		ConsecutiveOpens: 0,
		ConsecutiveAdds:  0,
		TradeHistory:     make([]TradeRecord, 0),
		LastResetTime:    time.Now(),
	}
	tl.symbolStats[symbol] = stats
	return stats
}

// 检查并重置每日统计（内部方法）
func (tl *TradeLimiter) checkAndResetDaily(stats *SymbolStats) {
	now := time.Now()
	lastReset := stats.LastResetTime

	// 计算今天的重置时间点
	todayReset := time.Date(now.Year(), now.Month(), now.Day(), tl.config.ResetHour, 0, 0, 0, now.Location())

	// 如果当前时间超过了今天的重置时间点，且上次重置时间在今天重置时间点之前
	if now.After(todayReset) && lastReset.Before(todayReset) {
		stats.TotalOpensToday = 0
		stats.TotalAddsToday = 0
		stats.LastResetTime = now
	}
}

// 使用示例
func main() {
	// 创建配置
	config := TradeLimiterConfig{
		MaxConsecutiveOpens: 2,                // 最多连续开仓2次
		MaxConsecutiveAdds:  3,                // 最多连续加仓3次
		OpenCooldownPeriod:  30 * time.Minute, // 开仓冷却30分钟
		AddCooldownPeriod:   15 * time.Minute, // 加仓冷却15分钟
		MaxOpensPerDay:      8,                // 每日最多开仓8次
		MaxAddsPerDay:       12,               // 每日最多加仓12次
	}

	// 创建限制器
	limiter := NewTradeLimiter(config)

	symbol := "BTCUSDT"

	// 检查是否可以开仓
	if limiter.CanOpen(symbol) {
		// 执行开仓
		limiter.RecordTrade(symbol, ActOpen, 50000.0, "LONG")
		println("开仓成功")
	}

	// 检查是否可以加仓
	if limiter.CanAdd(symbol) {
		// 执行加仓
		limiter.RecordTrade(symbol, ActAdd, 49800.0, "LONG")
		println("加仓成功")
	}

	// 记录平仓
	limiter.RecordClose(symbol, 51000.0, "LONG")

	// 查看统计信息
	stats := limiter.GetSymbolStats(symbol)
	if stats != nil {
		println("交易统计:", stats.Symbol, "连续开仓:", stats.ConsecutiveOpens, "连续加仓:", stats.ConsecutiveAdds)
	}
}
