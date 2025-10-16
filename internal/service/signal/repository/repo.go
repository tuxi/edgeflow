package repository

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"time"
)

// SignalRepository 负责最终信号的持久化（通常是 PostgreSQL/MySQL）
type SignalRepository interface {
	// 事务性地存储 Signal 及其关联的 TrendSnapshot
	SaveSignalWithSnapshot(ctx context.Context, signal *entity.Signal) error
	// 获取指定交易对的活跃信号列表 (用于信号列表页)
	GetActiveSignals(ctx context.Context, symbol string, limit int) ([]entity.Signal, error)
	// 根据给定的时间范围（包含开始和结束时间）查找特定交易对的所有信号
	GetSignalsByTimeRange(ctx context.Context, symbol string, start, end time.Time) ([]model.SignalHistory, error)
	// 查找特定ID的信号，并预加载其趋势快照 (用于信号详情页和下单)
	GetSignalDetailByID(ctx context.Context, id uint) (*model.SignalDetail, error)
	// 查询信号全部信息
	GetSignalByID(ctx context.Context, id uint) (*entity.Signal, error)
	// 获取信号列表
	GetAllActiveSignalList(ctx context.Context) ([]model.Signal, error)

	// 获取胜率
	GetSymbolWinRate(ctx context.Context, symbol string) (float64, error)
	// 计算给定交易对的总收益率百分比（FinalPnlPct 的总和）。
	GetSymbolTotalPnL(ctx context.Context, symbol string) (float64, error)
	// 在一次查询中获取交易对的聚合胜率、总收益率和总交易次数。
	GetSymbolPerformanceSummary(ctx context.Context, symbol string) (*model.PerformanceSummary, error)
}
