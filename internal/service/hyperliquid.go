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
	"log"
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
)

type HyperLiquidService struct {
	dao                        dao.HyperLiquidDao
	rc                         *redis.Client
	marketService              *MarketDataService
	isUpdatePositionsLoading   bool // 是否正在更细仓位信息
	updatePositionsLock        sync.Mutex
	leaderboardLock            sync.Mutex
	isUpdateLeaderboardLoading bool // 是否正在更新排行数据
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
					if err := h.fetchData(); err != nil {
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

func (h *HyperLiquidService) fetchData() error {
	rawData, err := h.fetchLeaderboard() // 调用 API 获取原始 leaderboard JSON
	if err != nil {
		log.Printf("HyperLiquidService fetchLeaderboard error: %v", err)
		return err
	}
	// 日活跃至少10万， 账户价值至少 100万，取前 100 名
	return h.updateWhaleLeaderboard(rawData, 100000.0, 1000000.0, 130)
}

func (h *HyperLiquidService) fetchLeaderboard() ([]types.TraderPerformance, error) {
	restClient, _ := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	data, err := restClient.LeaderboardCall()
	if err != nil {
		log.Printf("HyperLiquidService fetchLeaderboard error: %v", err)
		return nil, err
	}

	return data, nil
}

// 用户交易成交订单历史
func (h *HyperLiquidService) WhaleUserFillOrdersHistory(ctx context.Context, userAddress string) (orders []*types.UserFillOrder, err error) {
	rdsKey := consts.UserFillOrderKey + ":1:" + userAddress
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res []*types.UserFillOrder
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

	res, err = restClient.UserFillOrdersIn24Hours(ctx, userAddress)
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
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*30).Err()
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

	ctx := context.Background()
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
	topWhales, err := h.dao.GetTopWhales(ctx, "account_value", 120)

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
