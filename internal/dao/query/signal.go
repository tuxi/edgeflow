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

func (r *signalDao) GetAllActiveSignalList(ctx context.Context) ([]model.Signal, error) {
	var signals []model.Signal
	err := r.db.WithContext(ctx).Model(&model.Signal{}).Where("status = ?", "ACTIVE").Find(&signals).Error
	return signals, err
}
