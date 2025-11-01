package service

import (
	"context"
	"edgeflow/internal/consts"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
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
	"strconv"
	"time"
)

type HyperLiquidService struct {
	dao           dao.HyperLiquidDao
	rc            *redis.Client
	marketService *MarketDataService
}

func NewHyperLiquidService(dao dao.HyperLiquidDao, rc *redis.Client, marketService *MarketDataService) *HyperLiquidService {
	return &HyperLiquidService{dao: dao, rc: rc, marketService: marketService}
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

// 查询用户收益数据
func (h *HyperLiquidService) GetWhalePortfolioInfoGetAddress(ctx context.Context, address string) (*model.HyperWhalePortfolioRes, error) {
	restClient, err := rest.NewHyperliquidRestClient(
		"https://api.hyperliquid.xyz",
		"https://stats-data.hyperliquid.xyz/Mainnet/leaderboard",
	)

	if err != nil {
		return nil, err
	}

	portfolio, err := restClient.WhalePortfolioInfo(ctx, address)
	if err != nil {
		return nil, err
	}

	var result model.HyperWhalePortfolioRes
	res, err := h.dao.WhaleLeaderBoardInfoGetByAddress(ctx, address)

	result.PortfolioData = struct {
		TotalPnl     []types.DataPoint `json:"totalPnl"`
		PerpPnl      []types.DataPoint `json:"perpPnl"`
		TotalBalance []types.DataPoint `json:"totalBalance"`
		PerpBalance  []types.DataPoint `json:"perpBalance"`
	}(struct {
		TotalPnl     []types.DataPoint
		PerpPnl      []types.DataPoint
		TotalBalance []types.DataPoint
		PerpBalance  []types.DataPoint
	}{TotalPnl: portfolio.Total.AllTime.Pnl, PerpPnl: portfolio.Perp.AllTime.Pnl, TotalBalance: portfolio.Total.AllTime.AccountValue, PerpBalance: portfolio.Perp.AllTime.AccountValue})

	result.DisplayName = res.DisplayName

	result.Address = address
	result.AccountValue = lastValue(portfolio.Total.AllTime.AccountValue)
	result.VlmMonth = portfolio.Total.Month.Vlm
	result.VlmDay = portfolio.Total.Day.Vlm
	result.VlmWeek = portfolio.Total.Week.Vlm
	result.VlmAll = portfolio.Total.AllTime.Vlm
	result.PnLDay = lastValue(portfolio.Total.Day.Pnl)
	result.PnLAll = lastValue(portfolio.Total.AllTime.Pnl)
	result.PnLMonth = lastValue(portfolio.Total.Month.Pnl)
	result.PnLWeek = lastValue(portfolio.Total.Week.Pnl)

	result.WinRateUpdatedAt = res.WinRateUpdatedAt
	result.WinRateCalculationStartTime = res.WinRateCalculationStartTime
	result.TotalAccumulatedTrades = res.TotalAccumulatedTrades
	result.TotalAccumulatedProfits = res.TotalAccumulatedProfits
	result.TotalAccumulatedPnL = res.TotalAccumulatedPnL

	fmt.Printf("Day PnL: %.2f\n")

	return &result, nil
}

func lastValue(arr []types.DataPoint) float64 {
	if len(arr) == 0 {
		return 0
	}
	return arr[len(arr)-1].Value
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

func (h *HyperLiquidService) GetWinRateLeaderboardFromDB(ctx context.Context, limit int64) (*model.CustomLeaderboardEntryDBRes, error) {
	items, err := h.dao.GetWinRateLeaderboard(ctx, int(limit))
	if err != nil {
		return nil, err
	}

	// 查询上次更新日期
	lastUpdate, err := h.rc.Get(ctx, consts.WhaleWinRateLastUpdatedKey).Int64()
	if err != nil {
		if err != redis.Nil {
			logger.Errorf("Redis连接异常:%v", err.Error())
		}
		return &model.CustomLeaderboardEntryDBRes{
			Data:                 items,
			LastUpdatedTimestamp: time.Now().UnixMilli(),
		}, nil
	} else {
		return &model.CustomLeaderboardEntryDBRes{
			Data:                 items,
			LastUpdatedTimestamp: lastUpdate,
		}, nil
	}
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

		if pos.Side == "long" {
			analysis.LongValue += posValue
			analysis.LongMargin += marginUsed
			analysis.LongPnl += upnl
			analysis.LongFundingFee += -fundingFee // 所有多头的资金费
			analysis.LongCount++
		} else if pos.Side == "short" {
			analysis.ShortValue += posValue
			analysis.ShortMargin += marginUsed
			analysis.ShortPnl += upnl
			analysis.ShortFundingFee += -fundingFee // 所有空头的资金费
			analysis.ShortCount++
		}

		/*
			    资金费的意义:
				1.当仓位为多头时，资金费率为正时 -> 支付资金费 (支出)，资金费率为负时，收取资金费 (收入)
				2.当仓位为空头时，资金费率为正时 -> 收取资金费 (收入)，资金费率为负时，支付资金费 (支出)
		*/

		analysis.TotalFundingFee += -fundingFee // 所有鲸鱼总资金费（收入为正，支出为负）

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
	err = h.rc.Set(ctx, rdsKey, bytes, time.Second*30).Err()
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
