package kline

import (
	"edgeflow/internal/exchange"
	"edgeflow/internal/model"
	"edgeflow/pkg/utils"
	model2 "github.com/nntaoli-project/goex/v2/model"
	"go.uber.org/multierr"
	"log"
	"sync"
	"time"
)

type KlineManager struct {
	cache   map[string]map[model2.KlinePeriod][]model.Kline // symbol -> timeframe -> klines
	mu      sync.RWMutex
	ex      exchange.Exchange
	symbols []string
	// 添加一个用于接收外部关闭信号的通道
	stopCh chan struct{}
}

func NewKlineManager(ex exchange.Exchange, symbols []string) *KlineManager {
	newSymbols := make([]string, len(symbols))
	for i, symbol := range symbols {
		newSymbols[i] = utils.FormatSymbol(symbol)
	}

	return &KlineManager{
		cache:   make(map[string]map[model2.KlinePeriod][]model.Kline),
		mu:      sync.RWMutex{},
		ex:      ex,
		symbols: newSymbols,
		stopCh:  make(chan struct{}),
	}
}

// Stop 发送停止信号给调度循环
func (km *KlineManager) Stop() {
	close(km.stopCh)
}

// 启动 Manager 并进行首次铺底和调度
func (km *KlineManager) Start(trendUpdateCh chan<- struct{}) {
	// --- 第一次启动时先拉取k线数据铺底 ---
	log.Println("[KlineManager 启动]初始化k线数据...")
	km.fetchKAllLines() // 模拟获取所有 K 线数据

	// 首次拉取k线数据后，发送通知给 Trend Manager (流水线第一步)
	select {
	case trendUpdateCh <- struct{}{}:
	default:
		log.Println("[KlineManager WARNING] 初始 Trend Manager 通知通道阻塞，跳过。")
	}

	// 启动长期循环调度
	go km.run(trendUpdateCh)
}

// run 包含精准 K 线调度的核心逻辑
func (km *KlineManager) run(trendUpdateCh chan<- struct{}) {
	log.Println("[KlineManager 运行]，进入精准调度循环")

	// 我们以最小周期（15m）为基准进行调度
	const scheduleInterval = 15 * time.Minute

	// 初始化下一次执行时间
	nextExecutionTime := km.calculateNextExecutionTime(scheduleInterval)
	log.Printf("[KlineManager] 首次调度目标时间: %s", nextExecutionTime.Format("15:04:05"))

	// 主调度循环
	for {
		// 1. 计算需要等待的时间
		now := time.Now()
		sleepDuration := nextExecutionTime.Sub(now)

		// 2. 使用 select + time.After 实现优雅等待和停止监听
		if sleepDuration > 0 {
			timer := time.NewTimer(sleepDuration)
			log.Printf("[KlineManager] 下一次调度时间：%s（等待 %s）", nextExecutionTime.Format("15:04:05"), sleepDuration)

			select {
			case <-timer.C:
				// 等待时间已到，继续执行
			case <-km.stopCh:
				// 收到停止信号，退出循环
				timer.Stop()
				log.Println("[KlineManager] 收到停止信号，退出调度循环。")
				return
			}
		} else {
			// 如果错过了调度时间点，记录警告
			log.Printf("[KlineManager WARNING] 调度错过，目标时间: %s。立即执行。", nextExecutionTime.Format("15:04:05"))
		}

		// 3. 执行核心工作：根据时间点拉取 K 线数据

		// 关键：基于 nextExecutionTime (精确的收盘时间点) 进行条件判断
		execTime := nextExecutionTime // 理论执行时间点

		// 15分钟 K线总是拉取
		km.fetchKlines(km.symbols, model2.Kline_15min)
		log.Printf("[KlineManager] 15m K线更新完毕。")

		// 30分钟 K线收盘：发生在 00 和 30 分
		if execTime.Minute()%30 == 0 {
			km.fetchKlines(km.symbols, model2.Kline_30min)
			log.Printf("[KlineManager] 30m K线更新完毕。")
		}

		// 1小时 K线收盘：发生在 00 分
		if execTime.Minute() == 0 {
			km.fetchKlines(km.symbols, model2.Kline_1h)
			log.Printf("[KlineManager] 1h K线更新完毕。")

			// 4小时 K线收盘：发生在 1h 整点且小时能被 4 整除
			if execTime.Hour()%4 == 0 {
				km.fetchKlines(km.symbols, model2.Kline_4h)
				log.Printf("[KlineManager] 4h K线更新完毕。")
			}
		}

		time.Sleep(time.Second * 5)
		// 4. 通知 Trend Manager (流水线第一步) 开始工作
		select {
		case trendUpdateCh <- struct{}{}:
			// 成功发送更新通知
		default:
			// 如果 trendUpdateCh 阻塞，则跳过通知，防止调度器卡住
			log.Println("[KlineManager WARNING] Trend Manager 通道阻塞，跳过通知。")
		}

		// 5. 更新下一次执行时间
		nextExecutionTime = km.calculateNextExecutionTime(scheduleInterval)
		log.Printf("[KlineManager] 下一次调度目标时间: %s", nextExecutionTime.Format("15:04:05"))
	}
}

// calculateNextExecutionTime 计算下一个 K 线收盘的精确时间点 (例如 15:00, 15:15, 15:30...)
// 此逻辑保持不变，确保了精确的 15 分钟对齐
func (km *KlineManager) calculateNextExecutionTime(interval time.Duration) time.Time {
	now := time.Now()
	// 计算当前时间相对于午夜 00:00 的秒数
	secondsSinceMidnight := now.Hour()*3600 + now.Minute()*60 + now.Second()

	// 计算下一次调度点相对于午夜的秒数
	intervalSeconds := int(interval.Seconds())
	nextSeconds := (secondsSinceMidnight/intervalSeconds + 1) * intervalSeconds

	// 计算下一次执行时间
	nextExecutionTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		Add(time.Duration(nextSeconds) * time.Second)

	// 如果计算出的时间比现在早（通常不会发生，但以防万一）
	if nextExecutionTime.Before(now) {
		nextExecutionTime = nextExecutionTime.Add(interval)
	}

	return nextExecutionTime
}

func (tm *KlineManager) fetchKAllLines() {
	tm.fetchKlines(tm.symbols, model2.Kline_15min)
	tm.fetchKlines(tm.symbols, model2.Kline_30min)
	tm.fetchKlines(tm.symbols, model2.Kline_4h)
	tm.fetchKlines(tm.symbols, model2.Kline_1h)
}

// 拉取并存储K线
func (tm *KlineManager) fetchKlines(symbols []string, timeframe model2.KlinePeriod) error {
	var errs error
	for _, symbol := range symbols {
		kLines, err := tm.ex.GetKlineRecords(symbol, timeframe, 210, 0, model.OrderTradeSwap, true)
		if err != nil {
			log.Printf("[TrendManager] fetch %v kline error for %s: %v", timeframe, symbol, err)
			errs = multierr.Append(errs, err) // 使用 multierror 记录所有错误
			continue
		}
		tm.mu.Lock()
		if _, ok := tm.cache[symbol]; !ok {
			m := make(map[model2.KlinePeriod][]model.Kline)
			tm.cache[symbol] = m
		}
		tm.cache[symbol][timeframe] = kLines
		tm.mu.Unlock()
		//log.Printf("[Trend] %s 更新 %s K线，条数=%d", symbol, timeframe, len(kLines))
	}
	return errs
}

// 获取缓存
func (km *KlineManager) Get(symbol string, period model2.KlinePeriod) ([]model.Kline, bool) {
	km.mu.RLock()
	defer km.mu.RUnlock()
	lines, ok := km.cache[symbol][period]
	return lines, ok
}
