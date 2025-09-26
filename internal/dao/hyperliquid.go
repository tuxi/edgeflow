package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type HyperLiquidDao interface {
	// 更新一条鲸鱼信息
	WhaleUpsert(ctx context.Context, w *entity.Whale) error
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
	GetTopWhalePositions(ctx context.Context, limit int) ([]*entity.HyperWhalePosition, error)
	// 获取仓位多空
	GetWhaleLongShortRatio(ctx context.Context) (*model.WhaleLongShortRatio, error)
	// 查询某个用户的排行数据
	WhaleLeaderBoardInfoGetByAddress(ctx context.Context, address string) (*model.HyperWhaleLeaderBoard, error)
}
