package query

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type hyperLiquidDao struct {
	db *gorm.DB
}

// NewhyperLiquidDao 创建 DAO
func NewHyperLiquidDao(db *gorm.DB) *hyperLiquidDao {
	return &hyperLiquidDao{
		db: db,
	}
}

// ---------------------------
// 插入或更新 Whale 基础信息
func (dao *hyperLiquidDao) WhaleUpsert(ctx context.Context, w *entity.Whale) error {
	if w == nil {
		return gorm.ErrInvalidData
	}
	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "address"}},
			UpdateAll: true, // 存在则更新
		}).
		Create(w).Error
}

// 批量 Upsert Whale
func (dao *hyperLiquidDao) WhaleUpsertBatch(ctx context.Context, whales []*entity.Whale) error {
	if len(whales) == 0 {
		return nil
	}
	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "address"}},
			UpdateAll: true,
		}).
		CreateInBatches(whales, 100).Error
}

// ---------------------------
// 插入或更新排行榜数据
func (dao *hyperLiquidDao) WhaleStatUpsert(ctx context.Context, ws *entity.HyperLiquidWhaleStat) error {
	if ws == nil {
		return gorm.ErrInvalidData
	}
	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "address"}},
			UpdateAll: true,
		}).
		Create(ws).Error
}

// 批量 Upsert 排行榜数据
func (dao *hyperLiquidDao) WhaleStatUpsertBatch(ctx context.Context, wsList []*entity.HyperLiquidWhaleStat) error {
	if len(wsList) == 0 {
		return nil
	}
	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "address"}},
			UpdateAll: true,
		}).
		CreateInBatches(wsList, 100).Error
}

// ---------------------------
// 获取前 N 名排行榜
// period 可选 "day", "week", "month", "all"
// limit 指定返回前几名
func (dao *hyperLiquidDao) GetTopWhales(ctx context.Context, period string, limit int) ([]*model.HyperWhaleLeaderBoard, error) {
	var whales []*model.HyperWhaleLeaderBoard
	if limit <= 0 {
		limit = 100
	}

	// 根据 period 选择排序字段
	var orderBy string
	switch period {
	case "day":
		orderBy = "pnl_day DESC"
	case "week":
		orderBy = "pnl_week DESC"
	case "month":
		orderBy = "pnl_month DESC"
	case "all":
		orderBy = "pnl_all_time DESC"
	default:
		orderBy = "pnl_all_time DESC"
	}

	err := dao.db.WithContext(ctx).
		Model(&entity.HyperLiquidWhaleStat{}).
		Joins("LEFT JOIN whale ON hyper_whale_leaderboard.address = whale.address").
		Select("hyper_whale_leaderboard.*, whale.display_name").
		Order(orderBy).
		Limit(limit).
		Find(&whales).Error

	return whales, err
}
