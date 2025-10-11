package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type SignalDao interface {

	// 事务性地存储 Signal 及其关联的 TrendSnapshot
	SaveSignalWithSnapshot(ctx context.Context, signal *entity.Signal) error
	// 获取指定交易对的活跃信号列表 (用于信号列表页)
	GetActiveSignals(ctx context.Context, symbol string, limit int) ([]entity.Signal, error)
	// 查找特定ID的信号，并预加载其趋势快照 (用于信号详情页和下单)
	GetSignalDetailByID(ctx context.Context, id uint) (*model.SignalDetail, error)
	// 获取信号列表
	GetAllActiveSignalList(ctx context.Context) ([]model.Signal, error)
}
