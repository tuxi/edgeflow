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
	"log"
	"math"
	"sort"
	"strconv"
	"time"
)

type HyperLiquidService struct {
	dao dao.HyperLiquidDao
	rc  *redis.Client
}

func NewHyperLiquidService(dao dao.HyperLiquidDao, rc *redis.Client) *HyperLiquidService {
	return &HyperLiquidService{dao: dao, rc: rc}
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

// 开启定时任务抓取hyper排行榜数据
func (h *HyperLiquidService) StartLeaderboardUpdater(interval time.Duration) {
	go func() {
		_ = h.fetchData()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err := h.fetchData()
				if err != nil {
					continue
				}
			}
		}
	}()
}

func (h *HyperLiquidService) fetchData() error {
	rawData, err := h.fetchLeaderboard() // 调用 API 获取原始 leaderboard JSON
	if err != nil {
		log.Printf("HyperLiquidService fetchLeaderboard error: %v", err)
		return err
	}
	// 日活跃至少10完， 账户价值至少 100万，取前 100 名
	return h.updateWhaleLeaderboard(rawData, 100000.0, 1000000.0, 100)
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

// 定时任务：每隔N分钟更新一次鲸鱼持仓
func (s *HyperLiquidService) StartScheduler(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for {
		select {
		case <-ticker.C:
			if err := s.updatePositions(ctx); err != nil {
				fmt.Println("HyperLiquidService updateSnapshots error:", err)
			}
		case <-ctx.Done():
			return
		}
	}
}

// 获取前100个鲸鱼地址的仓位
func (h *HyperLiquidService) updatePositions(ctx context.Context) error {
	// 1. 获取前100鲸鱼
	topWhales, err := h.dao.GetTopWhales(ctx, "pnl_day", 100)

	if err != nil {
		return err
	}

	var snapshots []*entity.HyperWhalePosition

	// 2. 遍历查询持仓 (clearinghouseState)
	for _, whale := range topWhales {
		state, err := h.WhaleAccountSummaryGet(ctx, whale)

		if err != nil {
			continue
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

			ps := &entity.HyperWhalePosition{
				Address:        whale,
				Coin:           pos.Position.Coin,
				Type:           pos.Type,
				EntryPx:        pos.Position.EntryPx,
				PositionValue:  pos.Position.PositionValue,
				Szi:            pos.Position.Szi,
				UnrealizedPnl:  pos.Position.UnrealizedPnl,
				ReturnOnEquity: pos.Position.ReturnOnEquity,
				LeverageType:   pos.Position.Leverage.Type,
				LeverageValue:  pos.Position.Leverage.Value,
				MarginUsed:     pos.Position.MarginUsed,
				FundingFee:     pos.Position.CumFunding.AllTime,
				LiquidationPx:  pos.Position.LiquidationPx,
				Side:           side,
				UpdatedAt:      now,
				CreatedAt:      now,
			}

			snapshots = append(snapshots, ps)
		}
	}

	// 3. 存数据库
	if len(snapshots) > 0 {
		if err := h.dao.CreatePositionInBatches(ctx, snapshots); err != nil {
			return err
		}
	}

	// 4.对仓位进行分析
	analyzePos := h.analyzePositions(snapshots, nil)

	// 把分析的结果缓存到redis
	rdsKey := consts.WhalePositionsAnalyze

	bytes, err := json.Marshal(&analyzePos)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return nil
	}

	// 存储redis中，1分钟过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Minute*10).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}
	return nil
}

func (h *HyperLiquidService) GetTopWhalePositions(ctx context.Context) ([]*entity.HyperWhalePosition, error) {
	return h.getTopWhalePositions(ctx, 100)
}

func (h *HyperLiquidService) getTopWhalePositions(ctx context.Context, limit int) ([]*entity.HyperWhalePosition, error) {
	// 先从redis缓存中查找
	rdsKey := fmt.Sprintf("%v:%v", consts.WhalePositionsTop, limit)
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res []*entity.HyperWhalePosition
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

	// 从数据库获取
	res, err = h.dao.GetTopWhalePositions(ctx, limit)
	if err != nil {
		log.Printf("HyperLiquidService GetTopWhalePositions error: %v", err)
		return nil, err
	}

	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*10).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}

	return res, nil
}

func (h *HyperLiquidService) GetLongShortRatio(ctx context.Context) (*model.WhaleLongShortRatio, error) {
	rdsKey := consts.WhaleLongShortRatio
	bytes, err := h.rc.Get(ctx, rdsKey).Bytes()

	var res *model.WhaleLongShortRatio
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

	res, err = h.dao.GetWhaleLongShortRatio(ctx)
	if err != nil {
		log.Printf("HyperLiquidService GetLongShortRatio error: %v", err)
		return nil, err
	}
	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*10).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}
	return res, err
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

	posList, err := h.getTopWhalePositions(ctx, 1000)
	if err != nil {
		return nil, err
	}
	res = h.analyzePositions(posList, nil)

	if err != nil {
		log.Printf("HyperLiquidService AnalyzeTopPositions error: %v", err)
		return nil, err
	}
	bytes, err = json.Marshal(&res)
	if err != nil {
		logger.Errorf("HyperLiquidService 存储redis失败：%v", err.Error())
		return &res, nil
	}

	// 存储redis中，30秒过期
	err = h.rc.Set(ctx, rdsKey, bytes, time.Minute*1).Err()
	if err != nil {
		logger.Errorf("HyperLiquidService存储Cache失败:%v", err.Error())

	}

	return &res, nil
}

// 对仓位进行分析
func (h *HyperLiquidService) analyzePositions(positions []*entity.HyperWhalePosition, currentPriceMap map[string]float64) model.WhalePositionAnalysis {
	var analysis model.WhalePositionAnalysis
	// 设定高风险的杠杆阈值，适用于 LiquidationPx 存在时的风险提示。

	for _, pos := range positions {
		// 基础汇总
		posValue, err := strconv.ParseFloat(pos.PositionValue, 64)
		if err != nil {
			logger.Errorf("Failed to parse PositionValue for position %s: %v", pos.ID, err)
			continue
		}
		marginUsed, err := strconv.ParseFloat(pos.MarginUsed, 64)
		if err != nil {
			logger.Errorf("Failed to parse MarginUsed for position %s: %v", pos.ID, err)
			continue
		}

		upnl, err := strconv.ParseFloat(pos.UnrealizedPnl, 64)
		if err != nil {
			logger.Errorf("Failed to parse UnrealizedPnl for position %s: %v", pos.ID, err)
			continue
		}
		fundingFee, err := strconv.ParseFloat(pos.FundingFee, 64)
		if err != nil {
			logger.Errorf("Failed to parse FundingFee for position %s: %v", pos.ID, err)
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
			currentPrice, _ = currentPriceMap[pos.Coin] // 假设 pos.Symbol 是币种标识符
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

	return analysis
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
