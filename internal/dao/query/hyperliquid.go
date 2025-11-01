package query

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
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
		CreateInBatches(whales, 60).Error
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
		CreateInBatches(wsList, 60).Error
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

	// 确保默认或核心查询是按资产价值排序
	if period == "" {
		// 如果用户未指定排序，默认按总资产价值排序
		period = "account_value"
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

		"vlm_day":       true,
		"vlm_week":      true,
		"vlm_month":     true,
		"vlm_all_time":  true,
		"account_value": true, // 资产
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

func (h *hyperLiquidDao) UpdateWhaleStatsWinRateBatch(ctx context.Context, results []model.WinRateResult) error {
	if len(results) == 0 {
		return nil
	}

	var arr []entity.HyperLiquidWhaleStat
	for _, stat := range results {
		t := time.UnixMilli(stat.MaxTime)
		arr = append(arr, entity.HyperLiquidWhaleStat{
			Address:          stat.Address,
			WinRateUpdatedAt: &t,
		})
	}

	// 2. 使用 OnConflict + DoUpdates 仅更新指定字段
	return h.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// 冲突依据字段
			Columns: []clause.Column{{Name: "address"}},
			// 使用 DoUpdates 明确指定要更新的字段
			DoUpdates: clause.AssignmentColumns([]string{"win_rate", "total_closed_trades", "win_rate_updated_at"}),
		}).
		CreateInBatches(arr, 60).Error
}

func (h *hyperLiquidDao) GetTopWhales(ctx context.Context, period string, limit int) (address []string, err error) {
	if limit <= 0 {
		limit = 100
	}

	// 确保默认或核心查询是按资产价值排序
	if period == "" {
		// 如果用户未指定排序，默认按总资产价值排序
		period = "account_value"
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

		"vlm_day":       true,
		"vlm_week":      true,
		"vlm_month":     true,
		"vlm_all_time":  true,
		"account_value": true, // 资产
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
		// SkipDefaultTransaction跳过默认的事物，省去了事务开启和提交的开销
		Session(&gorm.Session{SkipDefaultTransaction: true}).
		CreateInBatches(positions, 200).Error

	return err
}

func (s *hyperLiquidDao) GetTopWhalePositions(ctx context.Context, req model.WhalePositionFilterReq) (*model.WhalePositionFilterRes, error) {

	var positions []*entity.HyperWhalePosition
	var totalCount int64

	db := s.db.WithContext(ctx).
		Model(&entity.HyperWhalePosition{})

	// 动态构建where语句
	db = db.Scopes(
		filterBySymbol(req.Coin),
		filterBySide(req.Side),
		filterByPnlStatus(req.PnlStatus),
		filterByFundingFee(req.FundingFeeStatus),
	)

	//统计总数
	if err := db.Count(&totalCount).Error; err != nil {
		return nil, err
	}

	// 排序、分页、执行查询
	if req.Limit > 0 {
		db = db.Limit(req.Limit).Offset(req.Offset)
	}

	// 按仓位大小 (PositionValue) 降序排列
	db = db.Order("position_value DESC")

	if err := db.Find(&positions).Error; err != nil {
		return nil, err
	}

	return &model.WhalePositionFilterRes{
		Total:     totalCount,
		Positions: positions,
	}, nil
}

// 创建鲸鱼胜率快照
func (dao *hyperLiquidDao) CreateSnapshotAndUpdateStatsT(ctx context.Context, snapshot *entity.WhaleWinRateSnapshot, stat entity.HyperLiquidWhaleStat) error {
	// 事务：写入快照并更新主表基线
	err := dao.db.WithContext(ctx).
		Transaction(func(tx *gorm.DB) error {

			if snapshot != nil {
				// A. 写入快照表
				if err := tx.Create(&snapshot).Error; err != nil {
					return err
				}
			}

			// B. 更新主表基线时间
			if err := tx.Model(&entity.HyperLiquidWhaleStat{}).
				Where("address = ?", stat.Address).
				Update("win_rate_updated_at", time.Now()).
				Update("last_successful_time", stat.LastSuccessfulTime).
				Update("win_rate_calc_start_time", stat.WinRateCalculationStartTime).
				Update("total_accumulated_profits", stat.TotalAccumulatedProfits).
				Update("total_accumulated_pnl", stat.TotalAccumulatedPnL).
				Update("total_accumulated_trades", stat.TotalAccumulatedTrades).Error; err != nil {
				return err
			}
			return nil
		})
	return err
}

func (dao *hyperLiquidDao) UpdateWhaleLastSuccessfulWinRateTime(ctx context.Context, address string, last int64) error {
	err := dao.db.WithContext(ctx).
		Model(&entity.HyperLiquidWhaleStat{}).
		Where("address = ?", address).
		Update("last_successful_time", last).
		Update("win_rate_updated_at", time.Now()).
		Error

	return err
}

func (dao *hyperLiquidDao) GetWinRateLeaderboard(ctx context.Context, limit int) ([]model.WhaleRankResult, error) {
	if limit == 0 {
		limit = 100
	}
	// 由于表中没有win_rate字段，而win_rate是需要total_accumulated_profits / total_accumulated_trades计算得到的
	// 所以需要在数据库中使用 SQL 运行时计算
	// 使用 GORM 的 Select 结合表达式，在查询时计算 Win Rate
	// 计算 WinRate，并按其降序排序
	query := dao.db.WithContext(ctx).
		Model(&entity.HyperLiquidWhaleStat{}).
		// 使用 SQL 表达式计算胜率，并给它一个别名 win_rate
		Select("address, total_accumulated_trades, total_accumulated_profits, (CAST(total_accumulated_profits AS DECIMAL(10, 4)) / total_accumulated_trades) AS win_rate").
		Where("total_accumulated_trades > ?", 0). // 排除分母为 0 的情况
		Order("win_rate DESC").                   // 按计算出的字段降序排序
		Limit(limit)

	var stats []model.WhaleRankResult

	err := query.Find(&stats).Error
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// 必须在 DAO 层实现获取和更新 的方法
func (dao *hyperLiquidDao) GetWhaleStatByAddress(ctx context.Context, address string) (*entity.HyperLiquidWhaleStat, error) {
	var stat entity.HyperLiquidWhaleStat

	err := dao.db.WithContext(ctx).
		Where("address = ?", address).
		Find(&stat).Error // 默认查询所有列

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &entity.HyperLiquidWhaleStat{Address: address}, nil
		}
		return nil, err
	}
	return &stat, nil
}

// 筛选币种
func filterBySymbol(symbol string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if symbol != "" {
			// 假设数据库字段名为 'symbol'
			return db.Where("coin = ?", symbol)
		}
		return db
	}
}

// Scope 2: 筛选方向
func filterBySide(side string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		if side != "" {
			// 假设数据库字段名为 'side'
			return db.Where("side = ?", side)
		}
		return db
	}
}

// Scope 3: 筛选盈亏状态
func filterByPnlStatus(status string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		//  'funding_fee' (资金费)
		switch status {
		case "profit": // 盈利
			// 未实现盈亏 > 0
			return db.Where("unrealized_pnl > 0")
		case "loss": // 亏损
			// 未实现盈亏 < 0
			return db.Where("unrealized_pnl < 0")
		case "neutral": // 平衡 (通常忽略，但如果需要，可以设置一个非常小的范围)
			return db.Where("unrealized_pnl BETWEEN -0.01 AND 0.01")
		default:
			return db // 不筛选
		}
	}
}

// Scope 4: 筛选资金费
func filterByFundingFee(status string) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// 'unrealized_pnl' (未实现盈亏)
		switch status {
		case "profit": // 盈利
			// 未实现盈亏 > 0
			return db.Where("funding_fee > 0")
		case "loss": // 亏损
			// 未实现盈亏 < 0
			return db.Where("funding_fee < 0")
		case "neutral": // 平衡 (通常忽略，但如果需要，可以设置一个非常小的范围)
			return db.Where("funding_fee BETWEEN -0.01 AND 0.01")
		default:
			return db // 不筛选
		}
	}
}

func (h *hyperLiquidDao) periodOrderBy(period string) string {
	// 根据 period 选择排序字段

	orderBy := fmt.Sprintf("%s DESC", period)
	return orderBy
}

// 专门用于接收 SUM 聚合结果的结构体
type NDayAggregateResult struct {
	// 对应 sum(total_closed_trades) AS total_closed_trades
	TotalClosedTrades int64 `gorm:"column:total_closed_trades"`

	// 对应 sum(winning_trades) AS winning_trades
	WinningTrades int64 `gorm:"column:winning_trades"`

	// 对应 sum(total_pnl) AS total_pnl
	TotalPnL float64 `gorm:"column:total_pnl"`
}

// GetWhaleLastNDaysWinRate 函数：计算过去 N 天的胜率（N日胜率）
//
// 参数:
//
//	ctx: Context
//	address: 鲸鱼地址
//	n: 过去的天数 (例如 7 为周胜率, 30 为月胜率)
//
// 返回:
//
//	*WinRateResult: 包含 N 日聚合结果
//	error: 错误信息
func (dao *hyperLiquidDao) GetWhaleLastNDaysWinRate(ctx context.Context, address string, n int) (*model.WinRateResult, error) {

	nDaysAgo := time.Now().AddDate(0, 0, -n).Format("2006-01-02")

	// 使用修正后的结构体
	var result NDayAggregateResult

	// 修正：SELECT 语句使用精确的列名和别名，与 NDayAggregateResult 匹配
	err := dao.db.WithContext(ctx).
		Table("whale_daily_stats").
		Select("sum(total_closed_trades) as total_closed_trades, sum(winning_trades) as winning_trades, sum(total_pnl) as total_pnl").
		Where("address = ?", address).
		Where("stat_date >= ?", nDaysAgo).
		Scan(&result).Error

	if err != nil {
		return nil, fmt.Errorf("failed to retrieve %d-day stats from database: %w", n, err)
	}

	// 修正：使用 NDayAggregateResult 中的 TotalClosedTrades 和 WinningTrades
	winRate := 0.0
	if result.TotalClosedTrades > 0 {
		winRate = float64(result.WinningTrades) / float64(result.TotalClosedTrades)
	}

	return &model.WinRateResult{
		Address: address,
		WinRate: winRate * 100,
	}, nil
}
