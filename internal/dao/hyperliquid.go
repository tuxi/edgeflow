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
	GetTopWhales(ctx context.Context, period string, limit int) ([]*model.HyperWhaleLeaderBoard, error)
}
