package service

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/hype/rest"
	"edgeflow/pkg/hype/types"
	"log"
	"sort"
	"time"
)

type HyperLiquidService struct {
	dao dao.HyperLiquidDao
}

func NewHyperLiquidService(dao dao.HyperLiquidDao) *HyperLiquidService {
	return &HyperLiquidService{dao: dao}
}

func (h *HyperLiquidService) GetTopWhales(ctx context.Context, limit int) (*model.WhaleEntryListRes, error) {
	list, err := h.dao.GetTopWhales(ctx, "all", limit)
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
