package query

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"fmt"
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
func (dao *hyperLiquidDao) GetTopWhalesLeaderBoard(ctx context.Context, period string, limit int) ([]*model.HyperWhaleLeaderBoard, error) {
	var whales []*model.HyperWhaleLeaderBoard
	if limit <= 0 {
		limit = 100
	}

	if period == "" {
		period = "pnl_week"
	}

	if period == "all" {
		period = "all_time"
	}

	// 防止sql注入
	allowSortFileds := map[string]bool{
		"pnl_day":      true,
		"pnl_week":     true,
		"pnl_month":    true,
		"pnl_all_time": true,

		"roi_day":      true,
		"roi_week":     true,
		"roi_month":    true,
		"roi_all_time": true,

		"vlm_day":      true,
		"vlm_week":     true,
		"vlm_month":    true,
		"vlm_all_time": true,
	}

	if !allowSortFileds[period] {
		return nil, fmt.Errorf("invalid sort filed: %s", period)
	}

	// 根据 period 选择排序字段
	orderBy := dao.periodOrderBy(period)

	err := dao.db.WithContext(ctx).
		Model(&entity.HyperLiquidWhaleStat{}).
		Joins("LEFT JOIN whale ON hyper_whale_leaderboard.address = whale.address").
		Select("hyper_whale_leaderboard.*, whale.display_name").
		Order(orderBy).
		Limit(limit).
		Find(&whales).Error

	return whales, err
}

func (h *hyperLiquidDao) WhaleLeaderBoardInfoGetByAddress(ctx context.Context, address string) (*model.HyperWhaleLeaderBoard, error) {
	var whales *model.HyperWhaleLeaderBoard

	err := h.db.WithContext(ctx).
		Model(&entity.HyperLiquidWhaleStat{}).
		Where("hyper_whale_leaderboard.address = ?", address).
		// 注意：这里也需要根据实际情况调整 JOIN 条件，
		// 确保 JOIN 的是 hyper_whale_leaderboard 表
		Joins("LEFT JOIN whale ON hyper_whale_leaderboard.address = whale.address").
		Select("hyper_whale_leaderboard.*, whale.display_name").
		Find(&whales).Error

	return whales, err
}

func (h *hyperLiquidDao) GetTopWhales(ctx context.Context, period string, limit int) (address []string, err error) {
	if limit <= 0 {
		limit = 100
	}

	if period == "" {
		period = "pnl_week"
	}
	if period == "all" {
		period = "all_time"
	}

	allowSortFilleds := map[string]bool{
		"pnl_day":      true,
		"pnl_week":     true,
		"pnl_month":    true,
		"pnl_all_time": true,

		"roi_day":      true,
		"roi_week":     true,
		"roi_month":    true,
		"roi_all_time": true,

		"vlm_day":      true,
		"vlm_week":     true,
		"vlm_month":    true,
		"vlm_all_time": true,
	}

	// 防止sql注入
	if !allowSortFilleds[period] {
		return nil, fmt.Errorf("invalid sort filed: %s", period)
	}

	orderBy := h.periodOrderBy(period)

	err = h.db.WithContext(ctx).
		Model(&entity.HyperLiquidWhaleStat{}).
		Select("address").
		Order(orderBy).
		Limit(100).
		Pluck("address", &address).Error
	return
}

func (h *hyperLiquidDao) CreatePositionInBatches(ctx context.Context, positions []*entity.HyperWhalePosition) error {

	if len(positions) == 0 {
		return nil
	}

	err := h.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// 确保一个仓位唯一的关键要素：(address, coin, leverage_type, leverage_value)
			Columns: []clause.Column{
				{Name: "address"},
				{Name: "coin"},
				{Name: "leverage_value"},
				{Name: "leverage_type"},
			},
			UpdateAll: true, // 冲突时更新所有字段
		}).
		CreateInBatches(positions, 100).Error

	return err
}

func (s *hyperLiquidDao) GetTopWhalePositions(ctx context.Context, limit int) ([]*entity.HyperWhalePosition, error) {
	var positions []*entity.HyperWhalePosition
	err := s.db.WithContext(ctx).
		Order("position_value DESC"). // 按仓位价值降序
		Limit(limit).                 // 取前N名
		Find(&positions).Error
	if err != nil {
		return nil, err
	}
	return positions, nil
}

// 获取hyper鲸鱼的多空数量
func (s *hyperLiquidDao) GetWhaleLongShortRatio(ctx context.Context) (*model.WhaleLongShortRatio, error) {
	//查询逻辑是把所有仓位按方向分组求总价值
	//得到多仓总价值和空仓总价值后，计算多空比
	var res []model.WhaleLongShortResult
	err := s.db.WithContext(ctx).
		Model(&entity.HyperWhalePosition{}).
		// side as direction选取方向列，取别名direction
		// SUM(position_value) as total 将同一个方向的仓位价值累加
		Select("side as direction, SUM(position_value) as total").
		// 按仓位方向分组
		Group("side").
		Scan(&res).Error
	ratio := &model.WhaleLongShortRatio{}
	for _, r := range res {
		switch r.Direction {
		case "long":
			ratio.LongValue = r.Total
		case "short":
			ratio.ShortValue = r.Total
		}
	}
	if ratio.ShortValue > 0 {
		ratio.Ratio = ratio.LongValue / ratio.ShortValue
	} else {
		ratio.Ratio = 0
	}
	return ratio, err
}

func (h *hyperLiquidDao) periodOrderBy(period string) string {
	// 根据 period 选择排序字段

	orderBy := fmt.Sprintf("%s DESC", period)
	return orderBy
}
