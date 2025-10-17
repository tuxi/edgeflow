package query

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"math"
	"time"
)

type signalDao struct {
	db *gorm.DB
}

func NewSignalDao(db *gorm.DB) dao.SignalDao {
	return &signalDao{
		db: db,
	}
}

// SaveSignalWithSnapshot 事务性地存储 Signal 及其关联的 TrendSnapshot。
// 必须确保 model.Signal.TrendSnapshot 字段已被填充。
func (r *signalDao) SaveSignalWithSnapshot(ctx context.Context, signal *entity.Signal) error {
	if signal.TrendSnapshot == nil {
		return errors.New("cannot save signal: TrendSnapshot is missing")
	}

	// 1. 启动事务
	// **设计原则：Deadlock 错误应由调用方（Service Layer）自动重试。**
	// DAO 内部保证了最优的锁顺序 (SELECT FOR UPDATE -> INSERT -> UPDATE)。
	// 关键修正：使用 WithContext(ctx) 将上下文传递给事务，修复奇葩的死锁问题
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {

		// 强制重置 ID 和外键 (防御性措施) ---
		// 确保 GORM 知道我们要插入新的记录，而非更新或使用旧 ID。
		signal.ID = 0
		signal.TrendSnapshot.ID = 0
		signal.TrendSnapshot.SignalID = 0

		// --- 死锁修复：使用 SELECT FOR UPDATE 提前锁定旧的活跃信号 ---
		// 在执行任何 INSERT 之前，先获取对所有将被修改行的排它锁，防止死锁。
		// 我们需要锁定：相同 Symbol, 相同 signal_period, 且状态为 ACTIVE 的旧信号。
		if err := tx.Model(&entity.Signal{}).
			Where("symbol = ? AND signal_period = ? AND status = ?", signal.Symbol, signal.Period, "ACTIVE").
			Clauses(clause.Locking{Strength: "UPDATE"}).
			Find(&[]entity.Signal{}).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("failed to acquire locks on old signals: %w", err)
		}

		// 插入新的 Signal 记录 (包含 TrendSnapshot 的自动关联创建)
		// GORM 会自动：
		//   a) 插入 signals 记录，回填 signal.ID。
		//   b) 使用回填的 signal.ID 作为外键，插入 signal.TrendSnapshot 记录。
		if result := tx.Create(signal); result.Error != nil {
			return fmt.Errorf("failed to create signal: %w", result.Error)
		}

		// 尝试将同一 Symbol 的所有旧信号标记为 EXPIRED
		// 确保一个币种只有一个活跃信号
		if result := tx.Model(&entity.Signal{}).
			Where("symbol = ? AND signal_period = ? AND status = ? AND id != ?",
				signal.Symbol, signal.Period, "ACTIVE", signal.ID).
			Update("status", "EXPIRED"); result.Error != nil {
			return fmt.Errorf("failed to expire old signals: %w", result.Error)
		}

		// 如果没有错误，事务提交
		return nil
	})

	return err
}

// GetActiveSignals 获取指定交易对的活跃信号列表 (用于信号列表页)。
func (r *signalDao) GetActiveSignals(ctx context.Context, symbol string, limit int) ([]entity.Signal, error) {
	var signals []entity.Signal

	// 查询条件：指定 Symbol 和 ACTIVE 状态，并按时间倒序排列
	result := r.db.WithContext(ctx).Where("symbol = ? AND status = ?", symbol, "ACTIVE").
		Order("timestamp DESC").
		Limit(limit).
		Find(&signals)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to get active signals: %w", result.Error)
	}

	return signals, nil
}

func (r *signalDao) GetSignalsByTimeRange(ctx context.Context, symbol string, start, end time.Time) ([]model.SignalHistory, error) {
	var signals []model.SignalHistory
	result := r.db.WithContext(ctx).
		Where("symbol = ?", symbol).
		Where("timestamp >= ?", start). // 信号时间大于或等于起始时间
		Where("timestamp <= ?", end).   // 信号时间小于或等于结束时间
		Find(&signals)
	if result.Error != nil {
		return nil, fmt.Errorf("failed to retrieve signals for %s in time range %s to %s: %w",
			symbol, start.Format(time.RFC3339), end.Format(time.RFC3339), result.Error)
	}

	fmt.Printf("[DAO] Found %d signals for %s between %s and %s.\n",
		len(signals), symbol, start.Format("2006-01-02 15:04"), end.Format("2006-01-02 15:04"))

	return signals, nil
}

// GetSignalDetailByID 查找特定ID的信号，并预加载其趋势快照 (用于信号详情页和下单)。
func (r *signalDao) GetSignalDetailByID(ctx context.Context, id uint) (*model.SignalDetail, error) {
	var signal model.SignalDetail

	// 使用 Preload("TrendSnapshot") 自动 JOIN trend_snapshots 表并填充字段
	result := r.db.WithContext(ctx).Preload("TrendSnapshot").
		Where("id = ?", id).
		First(&signal)

	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, nil // 记录未找到，返回 nil
	}
	if result.Error != nil {
		return nil, fmt.Errorf("failed to get signal detail: %w", result.Error)
	}

	return &signal, nil
}

func (r *signalDao) GetSignalByID(ctx context.Context, id uint) (*entity.Signal, error) {
	var signal entity.Signal
	result := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&signal)

	if result.Error != nil {
		return nil, fmt.Errorf("failed to get signal detail: %w", result.Error)
	}

	return &signal, nil
}

func (r *signalDao) GetAllActiveSignalList(ctx context.Context) ([]model.Signal, error) {
	var signals []model.Signal
	err := r.db.WithContext(ctx).Model(&model.Signal{}).Where("status = ?", "ACTIVE").Find(&signals).Error
	return signals, err
}

// 将已完成信号的模拟结果保存到signal_outcomes表中。
// 当信号到达TP、SL或到期时，SignalTracker通常会调用此功能。
func (dao *signalDao) SaveSignalOutcome(ctx context.Context, outcome *entity.SignalOutcome) error {
	// GORM's Create method handles the insertion.
	// SignalID 字段上的 UNIQUE 约束将防止重复记录。
	// 在插入前确保 ClosedAt 字段被设置，通常在 SignalTracker 中完成。
	if outcome.ClosedAt.IsZero() {
		outcome.ClosedAt = time.Now()
	}

	result := dao.db.WithContext(ctx).Create(outcome)

	if result.Error != nil {
		// 检查是否为 UNIQUE 约束错误，如果信号结果已存在，可以忽略此错误（取决于具体业务需求）
		return fmt.Errorf("failed to save signal outcome for signal ID %d: %w", outcome.SignalID, result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no rows affected when saving signal outcome for ID %d", outcome.SignalID)
	}

	fmt.Printf("[signalDao] Successfully saved signal outcome for ID: %d, Outcome: %s\n", outcome.SignalID, outcome.Outcome)
	return nil
}

// 一个辅助结构，用于扫描数据库中的聚合结果。
// WinRateResult 是用于接收聚合查询结果的辅助结构。
type WinRateResult struct {
	TotalClosedSignals int64
	TotalWins          int64
}

// GetSymbolWinRate calculates the historical strategy win rate percentage for a given symbol.
// Win Rate = (Number of HIT_TPs / Total Completed Trades) * 100
// GetSymbolWinRate 计算给定交易对的历史策略胜率 (盈亏率百分比)。
func (dao *signalDao) GetSymbolWinRate(ctx context.Context, symbol string) (float64, error) {
	var result WinRateResult

	// 使用原生 SQL 查询进行高效聚合。
	// 胜率基于所有已平仓交易 (HIT_TP, HIT_SL, EXPIRED) 计算。
	query := `
		SELECT
			COUNT(id) AS total_closed_signals,
			COUNT(CASE WHEN outcome = ? THEN 1 END) AS total_wins
		FROM
			signal_outcomes
		WHERE
			symbol = ?`

	// 执行查询，使用 models 包中的常量 models.HitTP
	dbResult := dao.db.WithContext(ctx).Raw(query, entity.HitTP, symbol).Scan(&result)

	if dbResult.Error != nil {
		return 0, fmt.Errorf("failed to aggregate win rate for symbol %s: %w", symbol, dbResult.Error)
	}

	if result.TotalClosedSignals == 0 {
		return 0, nil // 没有已平仓交易，胜率为 0
	}

	// Calculate win rate percentage
	winRate := (float64(result.TotalWins) / float64(result.TotalClosedSignals)) * 100.0

	// 限制胜率范围在 [0, 100]
	winRate = math.Min(winRate, 100.0)
	winRate = math.Max(winRate, 0.0)

	fmt.Printf("[SignalDao] Calculated win rate for %s: Wins=%d, Total=%d, Rate=%.2f%%\n", symbol, result.TotalWins, result.TotalClosedSignals, winRate)

	return winRate, nil
}

// GetSymbolTotalPnL calculates the total PnL percentage (sum of FinalPnlPct) for a given symbol.
// This is the overall simulated return rate for the strategy on this symbol.
// GetSymbolTotalPnL 计算给定交易对的总收益率百分比（FinalPnlPct 的总和）。
func (dao *signalDao) GetSymbolTotalPnL(ctx context.Context, symbol string) (float64, error) {
	var totalPnL float64

	// 使用原生 SQL 查询，对指定交易对的所有 FinalPnlPct 字段求和。
	// COALESCE(SUM(...), 0) 用于处理没有记录时返回 0，而非 NULL。
	query := `
		SELECT
			COALESCE(SUM(final_pnl_pct), 0)
		FROM
			signal_outcomes
		WHERE
			symbol = ?`

	dbResult := dao.db.WithContext(ctx).Raw(query, symbol).Scan(&totalPnL)

	if dbResult.Error != nil {
		return 0, fmt.Errorf("failed to aggregate total PnL for symbol %s: %w", symbol, dbResult.Error)
	}

	// totalPnL 已经是百分比的累加值（例如 0.15 代表 15% 的总收益率）
	// 通常在展示层会将其乘以 100。
	fmt.Printf("[SignalDAO] Calculated total PnL for %s: %.4f\n", symbol, totalPnL)

	return totalPnL, nil
}

// GetSymbolPerformanceSummary fetches the aggregated win rate, total PnL, and total trade count for the symbol in a single query.
// GetSymbolPerformanceSummary 在一次查询中获取交易对的聚合胜率、总收益率和总交易次数。
func (dao *signalDao) GetSymbolPerformanceSummary(ctx context.Context, symbol string) (*model.PerformanceSummary, error) {
	var summary model.PerformanceSummary

	// 使用原生 SQL 查询，一次性计算所有指标
	query := `
		SELECT
			COUNT(id) AS total_closed_signals,
			COALESCE(SUM(final_pnl_pct), 0) AS total_pnl,
			COUNT(CASE WHEN outcome = ? THEN 1 END) AS total_wins
		FROM
			signal_outcomes
		WHERE
			symbol = ?`

	var temp struct {
		TotalClosedSignals int64   `gorm:"column:total_closed_signals"`
		TotalWins          int64   `gorm:"column:total_wins"`
		TotalPnL           float64 `gorm:"column:total_pnl"`
	}

	dbResult := dao.db.WithContext(ctx).Raw(query, entity.HitTP, symbol).Scan(&temp)

	if dbResult.Error != nil {
		return nil, fmt.Errorf("failed to aggregate performance summary for symbol %s: %w", symbol, dbResult.Error)
	}

	summary.TotalClosedSignals = temp.TotalClosedSignals
	summary.TotalPnL = temp.TotalPnL // 已经是累加值

	// 计算胜率
	if temp.TotalClosedSignals > 0 {
		winRate := (float64(temp.TotalWins) / float64(temp.TotalClosedSignals)) * 100.0
		// 确保胜率在 [0, 100]
		summary.WinRate = math.Min(winRate, 100.0)
		summary.WinRate = math.Max(summary.WinRate, 0.0)
	}

	//fmt.Printf("[DAO] Calculated performance summary for %s: WinRate=%.2f%%, TotalPnL=%.4f, TotalTrades=%d\n",
	//	symbol, summary.WinRate, summary.TotalPnL, summary.TotalClosedSignals)

	return &summary, nil
}
