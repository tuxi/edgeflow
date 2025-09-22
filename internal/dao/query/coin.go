package query

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"gorm.io/gorm"
)

type coinDao struct {
	db *gorm.DB
}

func NewCoinDao(db *gorm.DB) *coinDao {
	return &coinDao{db: db}
}

func (c *coinDao) CoinCreateNew(ctx context.Context, coin *entity.Coin) error {
	return c.db.WithContext(ctx).Create(coin).Error
}

func (c *coinDao) CoinUpdate(ctx context.Context, coin *entity.Coin) error {
	return c.db.WithContext(ctx).Updates(coin).Error
}

func (c *coinDao) CoinGetById(ctx context.Context, coinId int64) (res model.CoinOne, err error) {
	err = c.db.WithContext(ctx).Where("id = ?", coinId).Find(&res).Error
	return
}

func (c *coinDao) CoinGetByCoin(ctx context.Context, coin string) (res model.CoinOne, err error) {
	err = c.db.WithContext(ctx).Where("coin = ?", coin).Find(&res).Error
	return
}

func (c *coinDao) CoinGetListByCategory(ctx context.Context, categoryId int64, page, limit int) ([]model.CoinOne, error) {
	offset := (page - 1) * limit
	var arr []model.CoinOne
	err := c.db.WithContext(ctx).Model(&entity.Coin{}).Where("category_id = ?", categoryId).
		Where("status = ? ", 1).
		Order("id").
		Limit(limit).
		Offset(offset).
		Find(&arr).
		Error
	return arr, err
}
