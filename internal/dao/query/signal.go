package query

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"errors"
	"fmt"
	"gorm.io/gorm"
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
func (r *signalDao) SaveSignalWithSnapshot(signal *entity.Signal) error {
	if signal.TrendSnapshot == nil {
		return errors.New("cannot save signal: TrendSnapshot is missing")
	}

	// 1. 启动事务
	err := r.db.Transaction(func(tx *gorm.DB) error {

		// 2. 尝试将同一 Symbol 的所有旧信号标记为 EXPIRED
		// 这一步对于确保只有一个活跃信号是可选但推荐的。
		tx.Model(&entity.Signal{}).
			Where("symbol = ? AND status = ?", signal.Symbol, "ACTIVE").
			Update("status", "EXPIRED")

		// 3. 创建 Signal 记录。GORM 会自动处理 ID 回填。
		if result := tx.Create(signal); result.Error != nil {
			return fmt.Errorf("failed to create signal: %w", result.Error)
		}

		// 4. TrendSnapshot 关联写入
		// 由于 Signal.ID 已被回填，并且 TrendSnapshot 字段在 GORM Model 中已正确关联，
		// 我们可以设置 TrendSnapshot 的 SignalID 并写入。
		signal.TrendSnapshot.SignalID = signal.ID
		if result := tx.Create(signal.TrendSnapshot); result.Error != nil {
			return fmt.Errorf("failed to create trend snapshot: %w", result.Error)
		}

		// 如果没有错误，事务提交
		return nil
	})

	return err
}

// GetActiveSignals 获取指定交易对的活跃信号列表 (用于信号列表页)。
func (r *signalDao) GetActiveSignals(symbol string, limit int) ([]entity.Signal, error) {
	var signals []entity.Signal

	// 查询条件：指定 Symbol 和 ACTIVE 状态，并按时间倒序排列
	result := r.db.Where("symbol = ? AND status = ?", symbol, "ACTIVE").
		Order("timestamp DESC").
		Limit(limit).
		Find(&signals)

	if result.Error != nil && !errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to get active signals: %w", result.Error)
	}

	return signals, nil
}

// GetSignalDetailByID 查找特定ID的信号，并预加载其趋势快照 (用于信号详情页和下单)。
func (r *signalDao) GetSignalDetailByID(id uint) (*entity.Signal, error) {
	var signal entity.Signal

	// 使用 Preload("TrendSnapshot") 自动 JOIN trend_snapshots 表并填充字段
	result := r.db.Preload("TrendSnapshot").
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

func (r *signalDao) GetSignalList(ctx context.Context) ([]model.Signal, error) {
	var signals []model.Signal
	err := r.db.WithContext(ctx).Model(&model.Signal{}).Find(&signals).Error
	return signals, err
}
