package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type HyperLiquidDao interface {
	// 批量更新鲸鱼信息
	WhaleUpsertBatch(ctx context.Context, whales []*entity.Whale) error
	// 插入或更新排行榜数据
	WhaleStatUpsert(ctx context.Context, ws *entity.HyperLiquidWhaleStat) error
	// 批量插入或更新排行榜数据
	WhaleStatUpsertBatch(ctx context.Context, wsList []*entity.HyperLiquidWhaleStat) error

	// 获取前 N 名排行榜
	// period 可选 "day", "week", "month", "all"
	// limit 指定返回前几名
	GetTopWhalesLeaderBoard(ctx context.Context, period string, limit int) ([]*model.HyperWhaleLeaderBoard, error)
	// 获取前N名鲸鱼
	GetTopWhales(ctx context.Context, period string, limit int) ([]string, error)
	// 创建鲸鱼仓位快照
	CreatePositionInBatches(ctx context.Context, items []*entity.HyperWhalePosition) error
	// 获取前100仓位
	GetTopWhalePositions(ctx context.Context, req model.WhalePositionFilterReq) (*model.WhalePositionFilterRes, error)
	// 查询某个用户的排行数据
	WhaleLeaderBoardInfoGetByAddress(ctx context.Context, address string) (*model.HyperWhaleLeaderBoard, error)
	// 批量保存胜率到
	UpdateWhaleStatsWinRateBatch(ctx context.Context, results []model.WinRateResult) error
	// 写入鲸鱼胜率快照
	CreateSnapshotAndUpdateStatsT(ctx context.Context, snapshot *entity.WhaleWinRateSnapshot, stats entity.HyperLiquidWhaleStat) error
	// 根据地址获取鲸鱼
	GetWhaleStatByAddress(ctx context.Context, address string) (*entity.HyperLiquidWhaleStat, error)
	// 更新鲸鱼最后胜率时间
	UpdateWhaleLastSuccessfulWinRateTime(ctx context.Context, address string, last int64) error

	GetWinRateLeaderboard(ctx context.Context, limit int) ([]model.WhaleRankResult, error)
}
