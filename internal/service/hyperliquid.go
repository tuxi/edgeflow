package service

import (
	"context"
	"edgeflow/internal/consts"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/hype/rest"
	"edgeflow/pkg/hype/types"
	"edgeflow/pkg/logger"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"golang.org/x/time/rate"
	"log"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
)

// 更新鲸鱼胜率的固定时间为1小时一次
const updateWinRateTimeChunk = 1 * time.Hour

type HyperLiquidService struct {
	dao                        dao.HyperLiquidDao
	rc                         *redis.Client
	marketService              *MarketDataService
	isUpdatePositionsLoading   bool // 是否正在更细仓位信息
	updatePositionsLock        sync.Mutex
	leaderboardLock            sync.Mutex
	isUpdateLeaderboardLoading bool // 是否正在更新排行数据

	winRateLock          sync.Mutex // 胜率计算相关的锁和状态
	isWinRateCalculating bool       // 是否正在计算胜率
}

func NewHyperLiquidService(dao dao.HyperLiquidDao, rc *redis.Client, marketService *MarketDataService) *HyperLiquidService {
	return &HyperLiquidService{dao: dao, rc: rc, marketService: marketService}
}

// 定时任务：每隔N分钟更新一次鲸鱼持仓
func (s *HyperLiquidService) StartUpdatePositionScheduler(ctx context.Context, interval time.Duration) {
	go func() {
		timer := time.NewTimer(16)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				timer.Reset(interval)
				// 更新仓位信息是耗时操作，需要请求100个仓位接口的数据，也就是100次request
				s.updatePositionsLock.Lock()
				if s.isUpdatePositionsLoading {
					s.updatePositionsLock.Unlock()
					continue
				}
				s.isUpdatePositionsLoading = true
				s.updatePositionsLock.Unlock()

				// 启动一个独立goroutine来执行耗时操作和清理
				// 确保耗时操作不糊i阻塞ticker的下一次触发
				go func() {
					defer func() {
						s.updatePositionsLock.Lock()
						s.isUpdatePositionsLoading = false
						s.updatePositionsLock.Unlock()
					}()

					// 检查上下文是否已经被取消（虽然 ticker 也会响应 ctx.Done，但这里也做一次检查更安全）
					// if ctx.Err() != nil { return } // 实际应用中可以省略，因为 updatePositions 应该处理 ctx
					// 执行耗时的更新操作
					if err := s.updatePositions(ctx); err != nil {
						// 4. 错误处理使用 log.Printf 更规范
						log.Printf("HyperLiquidService updatePositions error: %v\n", err)
					}
				}()

			case <-ctx.Done():
				return
			}
		}
	}()
}

// 开启定时任务抓取hyper排行榜数据，所有的数据都是基于排行榜上的数据查询的
func (h *HyperLiquidService) StartLeaderboardUpdater(ctx context.Context, interval time.Duration) {
	go func() {
		// 创建一个定时器为5秒间隔，这样C通道会立即触发，实现立即执行首次任务
		timer := time.NewTimer(time.Second * 5)
		// 确保在函数退出时停止定时器，释放资源
		defer timer.Stop()

		for {
			select {
			// 监听Timer的通道
			case <-timer.C:
				// 立即重置Timer为设定的interval间隔
				timer.Reset(interval)
				h.leaderboardLock.Lock()
				if h.isUpdateLeaderboardLoading {
					h.leaderboardLock.Unlock()
					continue
				}
				h.isUpdateLeaderboardLoading = true
				h.leaderboardLock.Unlock()
				// 把耗时操作放入新的goroutine中执行
				go func() {
					defer func() {
						h.leaderboardLock.Lock()
						h.isUpdateLeaderboardLoading = false
						h.leaderboardLock.Unlock()
					}()
					if err := h.fetchLeaderboard(); err != nil {
						fmt.Printf("HyperLiquidService fetchData error: %v\n", err)
					}
				}()
			case <-ctx.Done(): // context的退出机制
				fmt.Println("HyperLiquidService LeaderboardUpdater stopped by context.")
				return
			}
		}
	}()
}

// 开启定时任务：每隔N小时更新一次鲸鱼胜率
func (h *HyperLiquidService) StartWinRateCalculator(ctx context.Context) {
	go func() {
		// 首次执行延迟，15 分钟后
		timer := time.NewTimer(time.Minute * 2)
		defer timer.Stop()

		for {
			select {
			case <-timer.C:
				timer.Reset(updateWinRateTimeChunk)

				h.winRateLock.Lock()
				if h.isWinRateCalculating {
					h.winRateLock.Unlock()
					continue
				}
				h.isWinRateCalculating = true
				h.winRateLock.Unlock()

				go func() {
					defer func() {
						h.winRateLock.Lock()
						h.isWinRateCalculating = false
						h.winRateLock.Unlock()
					}()

					// 使用新的 Context，确保 DB 和 API 调用有超时，2小时内成功就行
					winRateCtx, cancel := context.WithTimeout(context.Background(), 1*time.Hour)
					defer cancel()

					if err := h.updateWhaleWinRates(winRateCtx); err != nil {
						log.Printf("HyperLiquidService updateWhaleWinRates error: %v\n", err)
					}
				}()
			case <-ctx.Done():
				fmt.Println("HyperLiquidService WinRateCalculator stopped by context.")
				return
			}
		}
	}()
}

func (h *HyperLiquidService) WhaleAccountSummaryGet(ctx context.Context, address string) (*types.MarginData, error) {

	// 先从redis缓存中查找
	rdsKey := consts.WhaleAccountSummaryKey + ":1:" + address
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res types.MarginData
	if err == nil {
		err = json.Unmarshal(bytes, &res)
		if err == nil {
			return &res, nil
		}
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
	}

	restClient, err := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	if err != nil {
		return nil, err
	}

	res, err = restClient.PerpetualsAccountSummary(ctx, address)
	if err != nil {
		return nil, err
	}

	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return &res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*15).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}
	return &res, nil
}

// 查询用户在排行榜中的收益数据
func (h *HyperLiquidService) WhaleLeaderBoardInfoGetByAddress(ctx context.Context, address string) (*model.HyperWhaleLeaderBoard, error) {

	// 先从redis缓存中查找
	rdsKey := consts.HyperWhaleLeaderBoardInfoByAddress + ":1:" + address
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res *model.HyperWhaleLeaderBoard
	if err == nil {
		err = json.Unmarshal(bytes, &res)
		if err == nil {
			return res, nil
		}
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
	}

	res, err = h.dao.WhaleLeaderBoardInfoGetByAddress(ctx, address)

	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*15).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}
	return res, nil
}

func (h *HyperLiquidService) GetTopWhales(ctx context.Context, limit int, datePeriod, filterPeriod string) (*model.WhaleEntryListRes, error) {
	if datePeriod == "all" {
		datePeriod = "all_time"
	}
	if datePeriod == "" || filterPeriod == "" {
		return nil, errors.New("筛选条件不能空")
	}
	period := fmt.Sprintf("%v_%v", filterPeriod, datePeriod)
	if limit == 0 {
		limit = 100
	}
	list, err := h.dao.GetTopWhalesLeaderBoard(ctx, period, limit)
	if err != nil {
		return nil, err
	}

	res := &model.WhaleEntryListRes{
		Total: int64(len(list)),
		List:  list,
	}

	return res, nil
}

func (h *HyperLiquidService) fetchLeaderboard() error {
	restClient, _ := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	rawData, err := restClient.LeaderboardCall()
	if err != nil {
		log.Printf("HyperLiquidService fetchLeaderboard error: %v", err)
		return err
	}

	// 日活跃至少10万， 账户价值至少 100万，取前 100 名
	go func() {
		_ = h.updateWhaleLeaderboard(rawData, 100000.0, 1000000.0, 100)
	}()

	return nil
}

// 用户交易成交订单历史
func (h *HyperLiquidService) WhaleUserFillOrdersHistory(ctx context.Context, req model.HyperWhaleFillOrdersReq) (data *types.UserFillOrderData, err error) {
	if req.MaxLookbackDays <= 0 {
		req.MaxLookbackDays = 1
	}
	rdsKey := fmt.Sprintf("%s:%v:%v:%v", consts.UserFillOrderKey+":1:"+req.Address, req.Start, req.MaxLookbackDays, req.PrevWindowHours)
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res *types.UserFillOrderData
	if err == nil {
		err = json.Unmarshal(bytes, &res)
		if err == nil {
			return res, nil
		}
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
	}

	restClient, _ := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	res, err = restClient.UserFillOrdersIn24Hours(ctx, req.Address, req.Start, req.MaxLookbackDays, req.PrevWindowHours)
	if err != nil {
		log.Printf("HyperLiquidService fetchUserFillOrdersHistory error: %v", err)
		return nil, err
	}

	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*5).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}

	return res, nil
}

func (h *HyperLiquidService) WhaleUserOpenOrdersHistory(ctx context.Context, userAddress string) (orders []*types.UserOpenOrder, err error) {
	// 先从redis缓存中查找
	rdsKey := consts.UserOpenOrderKey + ":1:" + userAddress
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res []*types.UserOpenOrder
	if err == nil {
		err = json.Unmarshal(bytes, &res)
		if err == nil {
			return res, nil
		}
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
	}

	restClient, _ := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	res, err = restClient.UserOpenOrders(ctx, userAddress)
	if err != nil {
		log.Printf("HyperLiquidService fetchUserOpenOrdersHistory error: %v", err)
		return nil, err
	}

	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*30).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}

	return res, nil
}

// 获取鲸鱼转账、提现记录
func (h *HyperLiquidService) WhaleUserNonFundingLedgerGet(ctx context.Context, userAddress string) (orders []*types.UserNonFunding, err error) {
	// 先从redis缓存中查找
	rdsKey := consts.UserNonFundingLedger + ":1:" + userAddress
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res []*types.UserNonFunding
	if err == nil {
		err = json.Unmarshal(bytes, &res)
		if err == nil {
			return res, nil
		}
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
	}

	restClient, _ := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	res, err = restClient.UserNonFundingLedgerGet(ctx, userAddress)
	if err != nil {
		log.Printf("HyperLiquidService fetchUserOpenOrdersHistory error: %v", err)
		return nil, err
	}

	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*30).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}

	return res, nil
}

// 更新排行数据到数据库
func (h *HyperLiquidService) updateWhaleLeaderboard(rawLeaderboard []types.TraderPerformance, dayVlmThreshold float64, minAccountValue float64, topN int) error {
	if len(rawLeaderboard) == 0 {
		return nil
	}

	//dayVlmThreshold := 100000.0 // 日交易量阈值，可调整
	// 1️⃣ 筛选活跃鲸鱼
	var activeList []types.TraderPerformance
	// 筛查出活跃的鲸鱼，要求日活大于1000000
	for _, row := range rawLeaderboard {
		dayVlm := row.Day.Vlm
		// 筛选出日活跃，并且账户价值大的鲸鱼, 只保留活跃鲸鱼
		if dayVlm >= dayVlmThreshold && row.AccountValue >= minAccountValue {
			activeList = append(activeList, row)
		}
	}

	if len(activeList) == 0 {
		return nil
	}

	// 2️⃣ 按账户价值降序排序
	sort.Slice(activeList, func(i, j int) bool {
		return activeList[i].AccountValue > activeList[j].AccountValue
	})

	// 3️⃣ 截取前 topN
	if len(activeList) > topN {
		activeList = activeList[:topN]
	}

	// 4️⃣ 生成 Whale 和 WhaleStat 列表
	var whales []*entity.Whale
	var whaleStats []*entity.HyperLiquidWhaleStat
	now := time.Now()

	for _, row := range activeList {
		// Whale 基础信息
		whales = append(whales, &entity.Whale{
			Address:     row.EthAddress,
			DisplayName: row.DisplayName, // 如果 API 返回昵称
			UpdatedAt:   now,
		})

		item := entity.HyperLiquidWhaleStat{
			Address:      row.EthAddress,
			AccountValue: row.AccountValue,
			PnLDay:       row.Day.Pnl,
			PnLWeek:      row.Week.Pnl,
			PnLMonth:     row.Month.Pnl,
			PnLAll:       row.AllTime.Pnl,
			ROIDay:       row.Day.Roi,
			ROIWeek:      row.Week.Roi,
			ROIMonth:     row.Month.Roi,
			ROIAll:       row.AllTime.Roi,
			VlmDay:       row.Day.Vlm,
			VlmWeek:      row.Week.Vlm,
			VlmMonth:     row.Month.Vlm,
			VlmAll:       row.AllTime.Vlm,
			UpdatedAt:    now,
		}

		// WhaleStat 排行榜数据
		whaleStats = append(whaleStats, &item)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 5️⃣ 批量 Upsert Whale 基础信息
	if err := h.dao.WhaleUpsertBatch(ctx, whales); err != nil {
		return err
	}

	// 6️⃣ 批量 Upsert WhaleStat 排行榜数据
	return h.dao.WhaleStatUpsertBatch(ctx, whaleStats)
}

// 获取前100个鲸鱼地址的仓位
func (h *HyperLiquidService) updatePositions(ctx context.Context) error {

	// 1. 获取前100鲸鱼，真正的大鲸鱼日交易量很低的，这里使用账户资产查询，不使用交易量
	topWhales, err := h.dao.GetTopWhales(ctx, "account_value", 100)

	if err != nil {
		return err
	}

	var snapshots []*model.HyperWhalePosition
	var wg sync.WaitGroup
	// 用户保护snapshots 的并发写入
	var mu sync.Mutex
	var errs []error
	// 使用一个有容量的 channel 来限制并发数，例如10
	limitChan := make(chan struct{}, 10)

	// 2. 遍历查询持仓 (clearinghouseState)
	for _, item := range topWhales {
		wg.Add(1)
		limitChan <- struct{}{} // 占用一个并发槽

		go func(address string) {
			defer wg.Done()
			defer func() { <-limitChan }() // 释放并发槽

			state, err := h.WhaleAccountSummaryGet(ctx, address)

			if err != nil {
				// 错误处理
				mu.Lock()
				errs = append(errs, err)
				mu.Unlock()
				return
			}

			now := time.Now()
			for _, pos := range state.AssetPositions {

				side := "long"
				szi, _ := strconv.ParseFloat(pos.Position.Szi, 64)
				if szi < 0 {
					side = "short"
				}
				// 检查强平价格是否为空，如果为空设置为0，不然报错 Incorrect decimal value: '' for column 'liquidation_px' at row 8
				_, err := strconv.ParseFloat(pos.Position.LiquidationPx, 64)
				if err != nil {
					pos.Position.LiquidationPx = "0.0"
				}

				ps := &model.HyperWhalePosition{
					Address:        address,
					Coin:           pos.Position.Coin,
					Type:           pos.Type,
					EntryPx:        pos.Position.EntryPx,
					PositionValue:  pos.Position.PositionValue,
					Szi:            pos.Position.Szi,
					UnrealizedPnl:  pos.Position.UnrealizedPnl,
					ReturnOnEquity: pos.Position.ReturnOnEquity,
					LeverageType:   pos.Position.Leverage.Type,
					LeverageValue:  fmt.Sprintf("%d", pos.Position.Leverage.Value),
					MarginUsed:     pos.Position.MarginUsed,
					FundingFee:     pos.Position.CumFunding.AllTime,
					LiquidationPx:  pos.Position.LiquidationPx,
					Side:           side,
					UpdatedAt:      now,
					CreatedAt:      now,
				}

				mu.Lock()
				snapshots = append(snapshots, ps)
				mu.Unlock()
			}
		}(item)
	}

	// 等待并发任务完成
	wg.Wait()

	// 3. 存数据库
	//if len(snapshots) > 0 {
	//	if err := h.dao.CreatePositionInBatches(ctx, snapshots); err != nil {
	//		return err
	//	}
	//}

	if len(snapshots) == 0 {
		logger.Errorf("HyperLiquidService 没有获取到任何鲸鱼仓位")
		return nil
	}

	// 存入redis
	err = h.updatePositionsToRedis(ctx, snapshots)
	if err != nil {
		logger.Errorf("HyperLiquidService 仓位列表存储redis失败：%v", err.Error())
		return nil
	}

	// 对仓位进行分析
	h.analyzePositions(ctx, snapshots, h.marketService.GetPrices())

	return nil
}

// 更新鲸鱼胜率
func (h *HyperLiquidService) updateWhaleWinRates(ctx context.Context) error {

	const (
		// 配置：每日任务，总耗时不超过 1 小时
		TotalWhales       = 10000           // 假设需要计算 10000 个鲸鱼
		TargetDurationSec = 3600            // 目标时间 1 小时 = 3600 秒
		TargetRPS         = rate.Limit(0.2) // 目标速率：越 0.2 次/秒 (即 5 秒/次)，让等待更明显
		// 将初始令牌桶容量降至 1，迫使第二个请求开始就立即等待 5 秒
		BurstLimit = 1

		// 保护本地资源的并发限制
		ConcurrentLimit = 5
	)

	// 1. 获取需要计算胜率的鲸鱼地址列表
	topWhales, err := h.dao.GetTopWhales(ctx, "account_value", TotalWhales)
	if err != nil {
		return fmt.Errorf("HyperLiquidService 计算胜率中获取鲸鱼数据失败: %w", err)
	}

	// 如果没有鲸鱼，直接返回
	if len(topWhales) == 0 {
		return nil
	}

	// 2. 初始化速率限制器 (确保任务平稳在 1 小时内完成)
	limiter := rate.NewLimiter(TargetRPS, BurstLimit)

	var wg sync.WaitGroup

	// 用于保护本地并发数量的 Channel
	concurrentChan := make(chan struct{}, ConcurrentLimit)

	// 用于收集计算结果的 Channel (缓冲区设置为鲸鱼总数)
	results := make(chan *model.WinRateResult, len(topWhales))

	log.Printf("HyperLiquidService：开始计算%d个鲸鱼的获胜率，最大获胜率为%.2f RPS...", len(topWhales), TargetRPS)

	for _, address := range topWhales {
		// 增加计时和日志
		//waitStart := time.Now()
		// A. 速率控制 (令牌桶等待)
		// 阻塞，直到速率限制器允许发送下一个请求
		if err := limiter.Wait(ctx); err != nil {
			log.Printf("HyperLiquidService：WinRate任务在速率限制期间取消: %v", err)
			break // Context 被取消，退出循环
		}

		//waitDuration := time.Since(waitStart)
		// 打印等待时间超过 10 毫秒的请求，证明限速器正在阻塞主循环
		//if waitDuration > 10*time.Millisecond {
		//	log.Printf("HyperLiquidService [速率限制成功]%s已被阻止 %v", address, waitDuration)
		//}

		// B. 本地并发槽控制
		concurrentChan <- struct{}{}
		wg.Add(1)

		go func(addr string) {
			defer wg.Done()
			defer func() { <-concurrentChan }()

			// 内部包含指数退避重试的计算逻辑
			result, err := h.calculateIncrementalWinRate(ctx, addr)

			if err != nil {
				// 打印错误，但任务仍将 result (包含上次成功数据) 发送到 results Channel
				log.Printf("HyperLiquidService：未能计算获胜率 %s: %v", addr, err)
			}

			if err != nil {

			}

			// 无论成功与否，将结果发送到 Channel
			results <- result
		}(address)
	}

	// 3. 等待所有 Goroutine 完成
	wg.Wait()
	close(results)

	log.Println("HyperLiquidService：所有胜率计算Goroutines已完成。启动数据库和Redis更新。")

	// 4. 收集和准备最终数据
	var finalResults []model.WinRateResult
	var winRateMembers []*redis.Z

	for res := range results {
		// 确保地址不为空 (排除可能的零值结果)
		if res != nil && res.Address != "" {
			finalResults = append(finalResults, *res)

			// 只有当总交易笔数 > 0 时，才将其添加到 Redis ZSET 中进行排名
			if res.TotalTrades > 0 {
				winRateMembers = append(winRateMembers, &redis.Z{
					Score:  res.WinRate,
					Member: res.Address,
				})
			}
		}
	}

	// --- 批量更新 Redis ZSET 排行榜 ---
	// ⚠️ 修正：必须移除 DEL 命令！ZAdd 本身会覆盖已有的 Member，不会影响未更新的 Member。
	if len(winRateMembers) > 0 {
		if err := h.rc.ZAdd(ctx, consts.WhaleWinRateZSetKey, winRateMembers...).Err(); err != nil {
			// 这里的错误处理应该更细致
			return fmt.Errorf("failed to update Redis ZSET: %w", err)
		}
	}
	// 记录排行榜最后更新日期
	now := time.Now().UnixMilli()
	if err := h.rc.Set(ctx, consts.WhaleWinRateLastUpdatedKey, now, 0).Err(); err != nil {
		// 记录警告，如果失败不应中断整个任务，因为排行榜数据已经更新成功。
		log.Printf("HyperLiquidService Warning: 写入胜率更新时间到 Redis 失败: %v", err)
	}

	log.Printf("HyperLiquidService：已成功更新%v条DB和Redis ZSET中的%d鲸鱼获胜率。", len(winRateMembers), len(finalResults))

	return nil
}

// 从 Redis ZSET 中查询 Top N 我们自己计算的鲸鱼胜率排行榜。
func (h *HyperLiquidService) GetWinRateLeaderboard(ctx context.Context, limit int64) (*model.CustomLeaderboardEntryRes, error) {

	// 1. 使用 ZREVRANGE 命令从 ZSET 中按分数（胜率）降序获取成员和分数。
	// ZREVRANGE key start stop WITHSCORES
	// 0 是第一名，limit-1 是第 limit 名 (0-based index)
	zRangeArgs := h.rc.ZRevRangeWithScores(ctx, consts.WhaleWinRateZSetKey, 0, limit-1)

	// 2. 检查 Redis 错误
	result, err := zRangeArgs.Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // 列表为空
		}
		return nil, fmt.Errorf("redis ZREVRANGE failed: %w", err)
	}

	// 3. 转换结果为业务结构体
	leaderboard := make([]model.CustomLeaderboardEntry, len(result))
	for i, z := range result {
		// 由于是从 0 开始的 index，排名是 i + 1
		leaderboard[i] = model.CustomLeaderboardEntry{
			Rank:    i + 1,
			Address: z.Member.(string), // ZSET member 是鲸鱼地址
			WinRate: z.Score,           // ZSET score 是胜率
		}
	}

	// 查询上次更新日期
	lastUpdate, err := h.rc.Get(ctx, consts.WhaleWinRateLastUpdatedKey).Int64()
	if err != nil {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		return &model.CustomLeaderboardEntryRes{
			Data:                 leaderboard,
			LastUpdatedTimestamp: time.Now().UnixMilli(),
		}, nil
	} else {
		return &model.CustomLeaderboardEntryRes{
			Data:                 leaderboard,
			LastUpdatedTimestamp: lastUpdate,
		}, nil
	}
}

// 计算单个地址的增量胜率
func (h *HyperLiquidService) calculateIncrementalWinRate(ctx context.Context, address string) (*model.WinRateResult, error) {
	//  从 DB 读取当前态
	stats, err := h.dao.GetWhaleStatByAddress(ctx, address)
	if err != nil {
		return nil, fmt.Errorf("HyperLiquidService：查找鲸鱼状态失败: %w", err)
	}

	// 确定增量拉取基线 'since' 和 绝对基线 'AbsStart'
	const initialBacktrackDays = -10 // 初始回溯 10 天
	var initialBacktrackTime = time.Now().AddDate(0, 0, initialBacktrackDays).UnixMilli()

	// 是不是当前用户首次运行胜率计算
	isInitialRun := true
	if stats.WinRateCalculationStartTime != nil && *stats.WinRateCalculationStartTime > 0 {
		isInitialRun = false
	}

	// 本次拉取订单的开始时间
	var since int64
	// 上次如果有拉取过，就从上次时间开始拉取数据
	if stats.LastSuccessfulTime != nil {
		since = *stats.LastSuccessfulTime
	}

	// 首次运行，从回溯开始
	if isInitialRun {
		since = initialBacktrackTime
	}

	// 确定本次拉取窗口 [since, currentEnd]
	targetEnd := since + int64(updateWinRateTimeChunk/time.Millisecond)
	currentEnd := min(targetEnd, time.Now().UnixMilli()) // 目标是计算[start, end] 需要查询的订单窗口

	// 窗口已追平当前时间 (追平判断应该使用滚动基线)
	if stats.LastSuccessfulTime != nil && currentEnd <= *stats.LastSuccessfulTime {
		// 无需拉取，返回当前累计胜率
		return createResultFromStat(*stats, true), nil
	}

	// 拉取增量数据
	restClient, _ := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)
	// 支持 startTime 从上一次计算过的时间开始加载交易记录，这样只有第一次才会很慢
	trades, err := restClient.UserFillOrdersByWindow(ctx, address, since, currentEnd)
	if err != nil {
		log.Printf("HyperLiquidService 计算胜率查找交易记录失败 error: %v", err)
		// API 拉取失败，返回旧累计结果和错误
		return createResultFromStat(*stats, false), err
	}

	// --- 增量聚合 ---
	var deltaProfits int64 // 记录本次查询的交易记录中盈利次数
	var deltaTotals int64  // 记录本次查询的交易次数，已平仓次数
	var deltaPnL float64   // 本次查询的所有订单的总已实现盈利USDC，也可能为负亏损
	var lastSuccessfulTime int64
	if stats.LastSuccessfulTime != nil {
		lastSuccessfulTime = *stats.LastSuccessfulTime
	}
	// ⚠️ 注意：这里不再需要 maxTime，因为窗口由 currentEnd 决定。
	for _, trade := range trades {
		// 双重检查增量时间
		if trade.Time <= lastSuccessfulTime {
			continue
		}

		// 判断是否是平仓操作 (无论盈亏)
		if trade.IsClosed() {

			// 计入总交易次数 (所有平仓操作都算，包括强平、平本、平仓换向)
			deltaTotals++

			// 解析 PnL
			pnlValue, parseErr := strconv.ParseFloat(trade.ClosedPnl, 64)

			// 只有在解析成功时，才累加 PnL 和盈利次数
			if parseErr == nil {
				deltaPnL += pnlValue

				// 累加盈利次数
				if pnlValue > 0.0 {
					deltaProfits++
				}
			}
			// 注意：如果 parseErr != nil，我们仍然增加了 deltaTotals，但盈亏和盈利次数都不变。
			// 这意味着我们认为这是一个已平仓交易，但其 PnL 数据无效或缺失，这通常是 API 错误。
			// 为了健壮性，我们通常假设平仓交易的 PnL 字段是有效的。
		}
	}

	// 准备新的累计状态
	newStats := updateHyperWhaleStat(stats, deltaTotals, deltaProfits, deltaPnL, currentEnd)
	newStats.Address = address
	if isInitialRun {
		newStats.WinRateCalculationStartTime = &initialBacktrackTime // 首次运行时设置 AbsStart
	}

	// 处理三种场景：
	// 场景 3: 非首次运行，且没有新交易 (deltaTotals == 0)
	if deltaTotals == 0 && !isInitialRun {
		// 只更新 LastSuccessfulTime，将窗口推进，不更新累计总数，不创建快照。
		err := h.dao.UpdateWhaleLastSuccessfulWinRateTime(ctx, address, currentEnd)
		return createResultFromStat(newStats, err == nil), err
	}

	// 场景 1/2: 首次运行 (isInitialRun) 或 有新交易 (deltaTotals > 0)
	var snapshot *entity.WhaleWinRateSnapshot
	if deltaTotals > 0 {
		// 仅当有交易时才创建快照
		snapshot = &entity.WhaleWinRateSnapshot{
			Address:           address,
			StartTime:         time.UnixMilli(since),
			EndTime:           time.UnixMilli(currentEnd),
			TotalClosedTrades: int64(deltaTotals),
			WinningTrades:     int64(deltaProfits),
			TotalPnL:          deltaPnL,
		}
	}

	err = h.dao.CreateSnapshotAndUpdateStatsT(ctx, snapshot, newStats)

	// 返回最终结果
	return createResultFromStat(newStats, err == nil), err
}

// 计算新的累计状态
func updateHyperWhaleStat(oldStat *entity.HyperLiquidWhaleStat, dt, dp int64, dpnl float64, currentEnd int64) entity.HyperLiquidWhaleStat {
	newStat := *oldStat // 复制旧状态

	newStat.LastSuccessfulTime = &currentEnd
	totalAccumulatedTrades := dt
	if oldStat.TotalAccumulatedTrades != nil {
		totalAccumulatedTrades += *oldStat.TotalAccumulatedTrades
	}
	newStat.TotalAccumulatedTrades = &totalAccumulatedTrades

	totalAccumulatedProfits := dp
	if oldStat.TotalAccumulatedProfits != nil {
		totalAccumulatedProfits += *oldStat.TotalAccumulatedProfits
	}
	newStat.TotalAccumulatedProfits = &totalAccumulatedProfits

	totalAccumulatedPnL := dpnl
	if oldStat.TotalAccumulatedPnL != nil {
		totalAccumulatedPnL += *oldStat.TotalAccumulatedPnL
	}
	newStat.TotalAccumulatedPnL = &totalAccumulatedPnL
	now := time.Now()
	newStat.WinRateUpdatedAt = &now

	return newStat
}

// 创建最终返回的 WinRateResult
func createResultFromStat(stats entity.HyperLiquidWhaleStat, isSuccess bool) *model.WinRateResult {
	var totalTrades int64
	if stats.TotalAccumulatedTrades != nil {
		totalTrades = *stats.TotalAccumulatedTrades
	}
	var lastSuccessfulTime int64
	if stats.LastSuccessfulTime != nil {
		lastSuccessfulTime = *stats.LastSuccessfulTime
	}
	result := &model.WinRateResult{
		Address:     stats.Address,
		IsSuccess:   isSuccess,
		TotalTrades: totalTrades,
		MaxTime:     lastSuccessfulTime,
	}
	if stats.TotalAccumulatedTrades != nil && stats.TotalAccumulatedProfits != nil && *stats.TotalAccumulatedTrades > 0 {
		result.WinRate = (float64(*stats.TotalAccumulatedProfits) / float64(*stats.TotalAccumulatedTrades)) * 100.0
	}
	return result
}

func (h *HyperLiquidService) GetTopWhalePositions(ctx context.Context, req model.WhalePositionFilterReq) (*model.WhalePositionFilterRedisRes, error) {

	//res, err := h.dao.GetTopWhalePositions(ctx, req)

	res, err := h.GetTopWhalePositionsFromRedis(ctx, req)
	if err != nil {
		log.Printf("HyperLiquidService GetTopWhalePositions error: %v", err)
		return nil, err
	}

	return res, nil
}

// 获取最新仓位数据分析
func (h *HyperLiquidService) AnalyzeTopPositions(ctx context.Context) (*model.WhalePositionAnalysis, error) {
	rdsKey := consts.WhalePositionsAnalyze
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res model.WhalePositionAnalysis
	if err == nil {
		err = json.Unmarshal(bytes, &res)
		if err == nil {
			return &res, nil
		}
	} else {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
	}

	posList, err := h.GetTopWhalePositionsFromRedis(ctx, model.WhalePositionFilterReq{
		Coin:             "",
		Side:             "",
		PnlStatus:        "",
		FundingFeeStatus: "",
		Limit:            1000,
		Offset:           0,
	})
	if err != nil {
		log.Printf("HyperLiquidService AnalyzeTopPositions error: %v", err)
		return nil, err
	}
	if len(posList.Positions) == 0 {
		log.Printf("HyperLiquidService 拒绝分析仓位，没有获取到任何仓位")
		return nil, nil
	}

	res, err = h.analyzePositions(ctx, posList.Positions, h.marketService.GetPrices())

	if err != nil {
		log.Printf("HyperLiquidService AnalyzeTopPositions error: %v", err)
		return nil, err
	}
	return &res, nil
}

// 从数据库中获取仓位排行
func (h *HyperLiquidService) GetTopWhalePositionsFromDB(ctx context.Context) (*model.WhalePositionFilterRedisRes, error) {
	posList, err := h.GetTopWhalePositions(ctx, model.WhalePositionFilterReq{
		Coin:             "",
		Side:             "",
		PnlStatus:        "",
		FundingFeeStatus: "",
		Limit:            1000,
		Offset:           0,
	})
	if err != nil {
		return nil, err
	}
	return posList, nil
}

// 对仓位进行分析
func (h *HyperLiquidService) analyzePositions(ctx context.Context, positions []*model.HyperWhalePosition, currentPriceMap map[string]float64) (model.WhalePositionAnalysis, error) {
	var analysis model.WhalePositionAnalysis
	// 设定高风险的杠杆阈值，适用于 LiquidationPx 存在时的风险提示。

	for _, pos := range positions {
		// 基础汇总
		posValue, err := strconv.ParseFloat(pos.PositionValue, 64)
		if err != nil {
			logger.Errorf("Failed to parse PositionValue for position %s: %v", pos.PositionValue, err)
			continue
		}
		marginUsed, err := strconv.ParseFloat(pos.MarginUsed, 64)
		if err != nil {
			logger.Errorf("Failed to parse MarginUsed for position %s: %v", pos.PositionValue, err)
			continue
		}

		upnl, err := strconv.ParseFloat(pos.UnrealizedPnl, 64)
		if err != nil {
			logger.Errorf("Failed to parse UnrealizedPnl for position %s: %v", pos.PositionValue, err)
			continue
		}
		fundingFee, err := strconv.ParseFloat(pos.FundingFee, 64)
		if err != nil {
			logger.Errorf("Failed to parse FundingFee for position %s: %v", pos.PositionValue, err)
			continue
		}
		analysis.TotalValue += posValue
		analysis.TotalMargin += marginUsed
		analysis.TotalPnl += upnl
		analysis.TotalFundingFee += fundingFee

		if pos.Side == "long" {
			analysis.LongValue += posValue
			analysis.LongMargin += marginUsed
			analysis.LongPnl += upnl
			analysis.LongFundingFee += fundingFee
			analysis.LongCount++
		} else if pos.Side == "short" {
			analysis.ShortValue += posValue
			analysis.ShortMargin += marginUsed
			analysis.ShortPnl += upnl
			analysis.ShortFundingFee += fundingFee
			analysis.ShortCount++
		}

		liqPx, _ := strconv.ParseFloat(pos.LiquidationPx, 64)

		// 获取当前价格
		currentPrice := 0.0
		if currentPriceMap != nil {
			price, ok := currentPriceMap[pos.Coin] // 假设 pos.Symbol 是币种标识符
			if ok {
				currentPrice = price
			} else {
				symbol := fmt.Sprintf("%v-USDT", pos.Coin)
				price := currentPriceMap[symbol]
				currentPrice = price
			}

		}

		// 潜在爆仓判断
		// 高风险判断逻辑根据 MarginMode 区分 ✨
		isHighRisk := false

		// 核心判断：如果 LiquidationPx 为 0，说明很安全，不计入高风险
		if liqPx == 0 {
			isHighRisk = false // 明确标记为安全
		} else {
			// 存在当前币价，按照爆仓价格计算
			if currentPrice > 0 {
				distance := math.Abs(currentPrice - liqPx)
				riskPercentage := distance / currentPrice // 距离当前价格的百分比

				if riskPercentage < 0.05 {
					// 风险缓冲距离小于 5%，视为高风险
					isHighRisk = true
				}
			} else {
				// LiquidationPx 存在（非零），说明头寸存在强平风险线，必须进一步检查。
				if marginUsed > 0 && posValue > 0 {
					effectiveLeverage := posValue / marginUsed //计算杠杆倍数

					// 逐仓模式10x认为是高风险、全仓20仓认为是高杠杆
					if pos.LeverageType == "isolated" {
						// 逐仓：LiquidationPx 存在且杠杆高于阈值，则有风险。
						if effectiveLeverage >= 10 {
							isHighRisk = true
						}
					} else if pos.LeverageType == "cross" {
						// 全仓：LiquidationPx 存在且杠杆高于阈值，说明该头寸对整体账户风险贡献较大。
						if effectiveLeverage >= 20 {
							isHighRisk = true
						}
					}
				}
			}
		}

		if isHighRisk {
			analysis.HighRiskValue += posValue
		}
	}

	// 2. 平均值与杠杆：增加对 MarginUsed 为零的检查 (关键优化)
	if analysis.LongCount > 0 {
		analysis.LongAvgValue = analysis.LongValue / float64(analysis.LongCount)
		if analysis.LongMargin > 0 {
			analysis.LongAvgLeverage = analysis.LongValue / analysis.LongMargin
			// 建议重命名 LongProfitRate 为 LongPnLRatio 或 LongROI
			analysis.LongProfitRate = analysis.LongPnl / analysis.LongMargin
		}
	}

	if analysis.ShortCount > 0 {
		analysis.ShortAvgValue = analysis.ShortValue / float64(analysis.ShortCount)
		if analysis.ShortMargin > 0 {
			analysis.ShortAvgLeverage = analysis.ShortValue / analysis.ShortMargin
			analysis.ShortProfitRate = analysis.ShortPnl / analysis.ShortMargin
		}
	}

	// 3. 多空占比和倾斜指数
	if analysis.TotalValue > 0 {
		analysis.LongPercentage = analysis.LongValue / analysis.TotalValue
		analysis.ShortPercentage = analysis.ShortValue / analysis.TotalValue
		analysis.PositionSkew = (analysis.LongValue - analysis.ShortValue) / analysis.TotalValue
	}

	h.generateTradingSignal(&analysis)

	bytes, err := json.Marshal(&analysis)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return analysis, nil
	}
	// 把分析的结果缓存到redis
	rdsKey := consts.WhalePositionsAnalyze
	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Minute*1).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}

	return analysis, nil
}

// 生成合约开单方向建议
func (h *HyperLiquidService) generateTradingSignal(analysis *model.WhalePositionAnalysis) {
	score := 0.0

	// ----------------------------------------------------
	// 1. 仓位拥挤度 (权重 40%)
	// ----------------------------------------------------
	// 目标：做多 vs 做空拥挤度差异 (避免拥挤方向)
	// 偏多信号: 多头占比较低 (< 45%)
	if analysis.LongPercentage < 0.45 {
		score += 40.0 * (0.45 - analysis.LongPercentage) / 0.45 // 给予做多信号
	}
	// 偏空信号: 空头占比较高 (> 55%)
	if analysis.ShortPercentage > 0.55 {
		score -= 40.0 * (analysis.ShortPercentage - 0.55) / 0.45 // 给予做空信号
	}
	// 注：这里的 0.45 是归一化因子

	// ----------------------------------------------------
	// 2. 平均杠杆 (权重 30%)
	// ----------------------------------------------------
	// 目标：判断哪一方更激进 (激进一方通常是短期反转的燃料)

	// 偏多信号: 做空一方过于激进 (空头平均杠杆 > 15x)
	if analysis.ShortAvgLeverage > 15.0 {
		score += 30.0 * math.Min(1.0, (analysis.ShortAvgLeverage-15.0)/5.0)
	}
	// 偏空信号: 做多一方过于激进 (多头平均杠杆 > 15x)
	if analysis.LongAvgLeverage > 15.0 {
		score -= 30.0 * math.Min(1.0, (analysis.LongAvgLeverage-15.0)/5.0)
	}

	// ----------------------------------------------------
	// 3. 盈亏效率 (权重 30%)
	// ----------------------------------------------------
	// 目标：反转趋势 (如果一方正在亏损，趋势可能反转)

	// 偏多信号: 空头正在亏损 (Long PnL / Long Margin < 0)
	if analysis.ShortPnl < 0 {
		score += 30.0
	}
	// 偏空信号: 多头正在亏损 (Short PnL / Short Margin < 0)
	if analysis.LongPnl < 0 {
		score -= 30.0
	}

	// ----------------------------------------------------
	// 4. 结果汇总与建议
	// ----------------------------------------------------
	analysis.SignalScore = score

	if score >= 35 {
		analysis.SignalSuggestion = "强烈建议偏多 / 考虑平空"
	} else if score > 15 {
		analysis.SignalSuggestion = "建议偏多"
	} else if score <= -35 {
		analysis.SignalSuggestion = "强烈建议偏空 / 考虑平多"
	} else if score < -15 {
		analysis.SignalSuggestion = "建议偏空"
	} else {
		analysis.SignalSuggestion = "中性 / 观望"
	}
}

// 更新redis 中的仓位信息
// 原本是写入数据库的，但是需求要求每次分析必须是最新的仓位信息，所以没必要存储数据库了，也解决了每次几百条存储数据库耗时的问题
func (h *HyperLiquidService) updatePositionsToRedis(ctx context.Context, snapshots []*model.HyperWhalePosition) error {
	pipe := h.rc.Pipeline()

	// 1. 准备数据结构
	allValueZSet := "whale:pos:all:value"
	var allValueMembers []*redis.Z

	// 用于收集所有独立索引 ZSET 成员的 Map： key -> []*redis.Z
	// Key 格式例如: "idx:coin:ETH", "idx:side:long", "idx:pnl:profit", ...
	indexZSets := make(map[string][]*redis.Z)

	// 存储所有要删除的 Key (包括 Hash 详情和所有 ZSET)
	var keysToDelete []string

	// 为了简化，我们只删除和重新创建主 ZSET 和所有用到的索引 ZSET。
	// 实际中删除旧的 Hash 详情会更复杂（需要先读取旧 ZSET 的 ID）。
	// 但鉴于我们是全量快照，这里只管理 ZSET 的创建和删除。

	// 2. 遍历快照，收集数据到内存
	for _, pos := range snapshots {
		// 1) 仓位唯一 ID
		// 确保仓位id唯一
		posID := fmt.Sprintf("%s|%s|%s|%s", pos.Address, pos.Coin, pos.LeverageType, pos.LeverageValue)
		// 2) 准备 ZSET 成员
		val, _ := strconv.ParseFloat(pos.PositionValue, 64)
		z := &redis.Z{Score: val, Member: posID}

		// 3) 写入主排名 ZSET
		allValueMembers = append(allValueMembers, z)

		// 4) 写入详细数据 Hash
		detailKey := fmt.Sprintf("whale:pos:detail:%s", posID)
		posMap, _ := structToMap(pos) // 假设存在 structToMap 函数

		// 添加写入 Hash 的命令到 Pipeline
		// 注意：这里没有删除旧 Hash，下次写入会直接覆盖。
		pipe.HSet(ctx, detailKey, posMap)

		// 5) 写入 4 组独立索引 ZSET (重点)
		filters := map[string]string{
			"coin": pos.Coin,
			"side": pos.Side,
			"pnl":  pos.UnrealizedPnl,
			"fee":  pos.FundingFee,
		}

		for field, value := range filters {
			if value != "" {
				indexKey := fmt.Sprintf("idx:%s:%s", field, value)
				indexZSets[indexKey] = append(indexZSets[indexKey], z)
			}
		}
	}

	// 3. 批量写入 Pipeline

	// a. **主排名 ZSET 的处理**
	keysToDelete = append(keysToDelete, allValueZSet)
	pipe.Del(ctx, allValueZSet)
	pipe.ZAdd(ctx, allValueZSet, allValueMembers...)

	// b. **独立索引 ZSET 的处理**
	for key, members := range indexZSets {
		// 在删除列表中添加 Key
		keysToDelete = append(keysToDelete, key)

		// 删除旧的索引 ZSET
		pipe.Del(ctx, key)

		// 写入新的索引 ZSET
		pipe.ZAdd(ctx, key, members...)
	}

	// c. 执行 Pipeline
	_, err := pipe.Exec(ctx)
	if err != nil {
		// 如果写入失败，需要考虑如何处理，可能需要异步清理这次创建的临时 Hash 和 ZSET。
		// 但对于定时任务，简单返回错误即可。
		return fmt.Errorf("redis pipeline exec failed: %w", err)
	}

	return nil
}

// 从redis中获取仓位排行
func (s *HyperLiquidService) GetTopWhalePositionsFromRedis(ctx context.Context, req model.WhalePositionFilterReq) (*model.WhalePositionFilterRedisRes, error) {

	// 1. **构建交集 Key 列表**
	// 默认以排序 ZSET 作为第一个 Key，后续 Key 将与之求交集。
	intersectKeys := []string{"whale:pos:all:value"}

	// 针对每个请求的筛选条件，添加对应的 ZSET Key
	if req.Coin != "" {
		intersectKeys = append(intersectKeys, fmt.Sprintf("idx:coin:%s", req.Coin))
	}
	if req.Side != "" {
		intersectKeys = append(intersectKeys, fmt.Sprintf("idx:side:%s", req.Side))
	}
	if req.PnlStatus != "" {
		intersectKeys = append(intersectKeys, fmt.Sprintf("idx:pnl:%s", req.PnlStatus))
	}
	if req.FundingFeeStatus != "" {
		intersectKeys = append(intersectKeys, fmt.Sprintf("idx:fee:%s", req.FundingFeeStatus))
	}

	// 2. **执行 ZINTERSTORE 操作**
	// 获取唯一的临时 Key
	// 注意：INCR/DECR 操作会导致不是原子性的，使用单个操作，并在 defer 中清理更安全。
	// 实际生产中更推荐使用 UUID 或请求参数的哈希作为临时 Key。
	tempKey := fmt.Sprintf("temp:intersect:%s", uuid.New().String()) // 假设使用 UUID 生成唯一 key

	// **设置清理函数，确保临时 Key 总是被删除**
	// 在函数退出时（无论成功失败）执行
	defer s.rc.Del(ctx, tempKey)

	// 使用 ZINTERSTORE 计算交集
	zinterstoreCmd := s.rc.ZInterStore(ctx, tempKey, &redis.ZStore{
		Keys:      intersectKeys,
		Weights:   nil,
		Aggregate: "MAX",
	})

	// 3. **检查交集结果和总数**
	totalCount, err := zinterstoreCmd.Result()
	if err != nil {
		// ZINTERSTORE 执行失败，直接返回错误
		return nil, fmt.Errorf("redis ZInterStore failed: %w", err)
	}

	// 如果交集结果为空，提前返回
	if totalCount == 0 {
		return &model.WhalePositionFilterRedisRes{Total: 0, Positions: []*model.HyperWhalePosition{}}, nil
	}

	// 4. **获取排名和分页 (ZREVRANGE)**
	start := req.Offset
	end := req.Offset + req.Limit - 1
	positionIDs, err := s.rc.ZRevRange(ctx, tempKey, int64(start), int64(end)).Result()
	if err != nil {
		return nil, fmt.Errorf("redis ZRevRange failed: %w", err)
	}

	if len(positionIDs) == 0 {
		// 这里的 Total 仍然是 totalCount，但分页结果为空
		return &model.WhalePositionFilterRedisRes{Total: totalCount, Positions: []*model.HyperWhalePosition{}}, nil
	}

	// 5. **批量获取详细数据 (使用一个 Pipeline)**
	pipe := s.rc.Pipeline()

	// 构造所有 Hash Key
	detailKeys := make([]string, len(positionIDs))
	for i, id := range positionIDs {
		detailKeys[i] = fmt.Sprintf("whale:pos:detail:%s", id)
	}

	// 批量将 HGetAll 命令加入 Pipeline
	cmds := make([]*redis.StringStringMapCmd, len(detailKeys))
	for i, key := range detailKeys {
		cmds[i] = pipe.HGetAll(ctx, key)
	}

	// **执行 Pipeline**
	_, err = pipe.Exec(ctx)
	if err != nil {
		// Pipeline 执行失败
		return nil, fmt.Errorf("redis Pipeline exec failed: %w", err)
	}

	// 6. **结果反序列化**
	var positions []*model.HyperWhalePosition
	for _, cmd := range cmds {
		posMap := cmd.Val()
		// 假设 mapToPosition 存在且可以处理空值
		var pos model.HyperWhalePosition
		err := mapToStruct(posMap, &pos)
		if err == nil {
			positions = append(positions, &pos)
		}
	}

	return &model.WhalePositionFilterRedisRes{
		Total:     totalCount,
		Positions: positions,
	}, nil
}

// structToMap 将结构体转换为 map[string]interface{}
// 它使用 JSON marshal/unmarshal 实现，能正确处理嵌套结构和 JSON tags
func structToMap(obj interface{}) (map[string]interface{}, error) {
	if obj == nil {
		return nil, fmt.Errorf("input object is nil")
	}

	// 1. 结构体 -> JSON 字节
	data, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal struct to json: %w", err)
	}

	// 2. JSON 字节 -> map[string]interface{}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal json to map: %w", err)
	}
	stringMap := make(map[string]interface{})
	for k, v := range result {
		if v != nil {
			// 使用 fmt.Sprint 将 interface{} 转换为字符串
			stringMap[k] = fmt.Sprint(v)
		}
	}

	return stringMap, nil
}

// mapToPosition 将 map[string]string 转换回 结构体
func mapToStruct(data map[string]string, m interface{}) error {
	if data == nil || len(data) == 0 {
		return fmt.Errorf("input map is empty")
	}

	dataBytes, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal map to json: %w", err)
	}

	// 2. JSON 字节 -> *entity.HyperWhalePosition
	if err := json.Unmarshal(dataBytes, &m); err != nil {
		return fmt.Errorf("failed to unmarshal json to struct: %w", err)
	}

	return nil
}
