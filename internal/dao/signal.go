package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"time"
)

type SignalDao interface {

	// 事务性地存储 Signal 及其关联的 TrendSnapshot
	SaveSignalWithSnapshot(ctx context.Context, signal *entity.Signal) error
	// 获取指定交易对的活跃信号列表 (用于信号列表页)
	GetActiveSignals(ctx context.Context, symbol string, limit int) ([]entity.Signal, error)
	// 查找特定ID的信号，并预加载其趋势快照 (用于信号详情页和下单)
	GetSignalDetailByID(ctx context.Context, id uint) (*model.SignalDetail, error)
	GetSignalByID(ctx context.Context, id uint) (*entity.Signal, error)
	// 获取信号列表
	GetAllActiveSignalList(ctx context.Context) ([]model.Signal, error)
	// 根据给定的时间范围（包含开始和结束时间）查找特定交易对的所有信号
	GetSignalsByTimeRange(ctx context.Context, symbol string, start, end time.Time) ([]model.SignalHistory, error)

	// 保存一个信号的盈亏
	SaveSignalOutcome(ctx context.Context, outcome *entity.SignalOutcome) error
	// GetSymbolWinRate 计算给定交易对的历史策略胜率 (盈亏率百分比)。
	GetSymbolWinRate(ctx context.Context, symbol string) (float64, error)
	// 计算给定交易对的总收益率百分比（FinalPnlPct 的总和）。
	GetSymbolTotalPnL(ctx context.Context, symbol string) (float64, error)
	GetSymbolPerformanceSummary(ctx context.Context, symbol string) (*model.PerformanceSummary, error)
}
