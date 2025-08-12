package dao

import (
	"context"
	"edgeflow/internal/model"
	"gorm.io/gorm"
)

type OrderDao struct {
	db *gorm.DB
}

func NewOrderDao(db *gorm.DB) *OrderDao {
	return &OrderDao{db: db}
}

// 插入下单记录
func (d *OrderDao) InsertRiskRecord(ctx context.Context, record *model.OrderRecord) error {
	return d.db.WithContext(ctx).Create(record).Error
}

// 判断是否已存在
func (d *OrderDao) ExistsOrderHash(ctx context.Context, hash string) (bool, error) {
	var count int64
	err := d.db.WithContext(ctx).Model(&model.OrderRecord{}).Where("order_hash = ?", hash).Count(&count).Error
	return count > 0, err
}

// 查找相同策略下的最后一个订单
func (d *OrderDao) OrderGetLast(ctx context.Context, strategy string, symbol string, side string, td string) (or model.OrderRecord, err error) {
	err = d.db.WithContext(ctx).Model(&model.OrderRecord{}).
		Where("strategy = ?", strategy).
		Where("symbol = ?", symbol).
		Where("side = ?", side).
		Where("trade_type = ?", td).
		Order("created_at DESC").
		Limit(1).
		Find(&or).Error

	//err := d.db.Where("symbol = ? AND direction = ? AND strategy_id = ?", symbol, direction, strategyID).
	//Order("created_at DESC").Limit(1).First(&order).Error
	return
}

// 查找某个level下的最后一个订单
func (d *OrderDao) OrderGetByLevel(ctx context.Context, symbol string, level int, td string) (or model.OrderRecord, err error) {
	err = d.db.WithContext(ctx).Model(&model.OrderRecord{}).
		Where("level = ?", level).
		Where("symbol = ?", symbol).
		Where("trade_type = ?", td).
		Order("timestamp DESC").
		Limit(1).
		Find(&or).Error

	return
}
