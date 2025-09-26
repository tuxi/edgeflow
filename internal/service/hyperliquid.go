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
	"fmt"
	"github.com/go-redis/redis/v8"
	"log"
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

func (h *HyperLiquidService) GetTopWhales(ctx context.Context, limit int, period string) (*model.WhaleEntryListRes, error) {
	if period == "" {
		period = "all"
	}
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

// 拉取+存储
func (h *HyperLiquidService) updatePositions(ctx context.Context) error {
	// 1. 获取前100鲸鱼
	topWhales, err := h.dao.GetTopWhales(ctx, "pnl_day", 100)

	if err != nil {
		return err
	}

	var snapshots []entity.HyperWhalePosition

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

			ps := entity.HyperWhalePosition{
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

	return nil
}

func (h *HyperLiquidService) GetTopWhalePositions(ctx context.Context) ([]*entity.HyperWhalePosition, error) {
	// 先从redis缓存中查找
	rdsKey := consts.WhalePositionsTop100
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
	res, err = h.dao.GetTopWhalePositions(ctx, 50)
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
