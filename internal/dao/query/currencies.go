package query

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type currenciesDao struct {
	db *gorm.DB
}

func NewCurrenciesDao(db *gorm.DB) *currenciesDao {
	return &currenciesDao{db: db}
}

func (c *currenciesDao) CurrencyCreateNew(ctx context.Context, coin *entity.Currency) error {
	if coin == nil {
		return gorm.ErrInvalidData
	}
	return c.db.WithContext(ctx).Create(coin).Error
}

func (dao *currenciesDao) CurrencyUpsert(ctx context.Context, w *entity.Currency) error {

	if w == nil {
		return gorm.ErrInvalidData
	}
	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ccy"}},
			UpdateAll: true,
		}).
		Create(w).Error

}

// 批量 Upsert Whale
func (dao *currenciesDao) CurrencyUpsertBatch(ctx context.Context, currencies []*entity.Currency) error {
	if len(currencies) == 0 {
		return nil
	}
	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "ccy"}},
			UpdateAll: true,
		}).
		CreateInBatches(currencies, 100).Error
}

func (c *currenciesDao) CurrencyUpdate(ctx context.Context, coin *entity.Currency) error {
	if coin == nil {
		return gorm.ErrInvalidData
	}
	return c.db.WithContext(ctx).Updates(coin).Error
}

func (c *currenciesDao) CurrencyGetById(ctx context.Context, coinId int64) (res model.CurrencyOne, err error) {
	err = c.db.WithContext(ctx).Where("id = ?", coinId).Find(&res).Error
	return
}

func (c *currenciesDao) CurrencyGetByCcy(ctx context.Context, coin string) (res model.CurrencyOne, err error) {
	err = c.db.WithContext(ctx).Where("coin = ?", coin).Find(&res).Error
	return
}

func (c *currenciesDao) CurrencyGetListByExchange(ctx context.Context, exId int64, page, limit int) (total int64, list []model.CurrencyOne, err error) {
	offset := (page - 1) * limit

	// 先统计总数
	if err = c.db.WithContext(ctx).
		Table("currencies AS c").                                         // 设置currencies表的别名为c
		Joins("JOIN exchange_currencies AS ce ON ce.currency_id = c.id"). // 条件查询
		Where("ce.exchange_id = ? AND ce.is_active = ?", exId, true).
		Count(&total).Error; err != nil {
		return
	}

	// 查询分页数据
	if err = c.db.WithContext(ctx).
		Table("currencies AS c").
		//Select("c.id, c.ccy, c.name, c.name_en").
		Joins("JOIN exchange_currencies AS ce ON ce.currency_id = c.id").
		Where("ce.exchange_id = ? AND ce.is_active = ?", exId, true).
		Order("c.id").
		Limit(limit).
		Offset(offset).
		Scan(&list).Error; err != nil {
		return
	}

	return total, list, nil

}

// 创建一个货币，并关联到交易所
func (c *currenciesDao) CurrencyCreateNewWithExchange(ctx context.Context, exchangeId int64, currency *entity.Currency) error {

	err := c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 插入币种
		if err := tx.Create(currency).Error; err != nil {
			return err
		}

		// 2. 插入币种-交易所关联
		exchangeCurrency := &entity.CurrencyExchanges{
			CurrencyId: currency.Id,
			ExchangeId: exchangeId,
			IsActive:   true,
		}
		if err := tx.Create(exchangeCurrency).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	return nil
}

// 批量创建货币，并关联到某个交易所
func (c *currenciesDao) CurrencyCreateBatchWithExchange(ctx context.Context, exchangeId int64, currencies []entity.Currency) error {
	// 带有事物的
	return c.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// 1. 批量插入币种
		if err := tx.Create(&currencies).Error; err != nil {
			return err
		}

		// 2. 构建币种-交易所关联记录
		var exchangeRelations []entity.CurrencyExchanges
		for _, currency := range currencies {
			exchangeRelations = append(exchangeRelations, entity.CurrencyExchanges{
				CurrencyId: currency.Id,
				ExchangeId: exchangeId,
				IsActive:   true,
			})
		}

		// 3. 批量插入关联表
		if len(exchangeRelations) > 0 {
			if err := tx.Create(&exchangeRelations).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (c *currenciesDao) ExchangeCreateNew(ctx context.Context, name, nameEn string) (*model.Exchange, error) {
	var ex model.Exchange

	// 1. 查询是否存在
	err := c.db.WithContext(ctx).Where("name = ?", name).First(&ex).Error
	if err == nil {
		// 已存在，直接返回
		return &ex, nil
	}
	if err != gorm.ErrRecordNotFound {
		// 查询出错
		return nil, err
	}

	// 2. 不存在，则创建
	ex = model.Exchange{
		Name:   name,
		NameEn: nameEn,
	}
	if err := c.db.WithContext(ctx).Create(&ex).Error; err != nil {
		return nil, err
	}

	return &ex, nil
}

func (c *currenciesDao) ExchangesGet(ctx context.Context) ([]model.Exchange, error) {
	var exs []model.Exchange
	err := c.db.WithContext(ctx).Find(&exs).Error
	if err != nil {
		return nil, err
	}
	return exs, nil
}

// 某个币上线到交易所，把它关联到交易所
func (c *currenciesDao) AssociateCurrencyWithExchange(ctx context.Context, currencyId, exchangeId int64) error {
	// 先检查是否已经关联
	var exist entity.CurrencyExchanges
	err := c.db.WithContext(ctx).
		Where("currency_id = ? AND exchange_id = ?", currencyId, exchangeId).
		First(&exist).Error
	if err == nil {
		// 已经关联，无需重复创建
		return nil
	}
	if err != gorm.ErrRecordNotFound {
		return err
	}

	// 创建关联记录
	exchangeCurrency := &entity.CurrencyExchanges{
		CurrencyId: currencyId,
		ExchangeId: exchangeId,
		IsActive:   true,
	}

	return c.db.WithContext(ctx).Create(exchangeCurrency).Error
}

func (c *currenciesDao) AssociateCurrenciesWithExchangeBatch(ctx context.Context, currencyIds []int64, exchangeId int64) error {
	if len(currencyIds) == 0 {
		return nil
	}

	// 1. 查询已存在的关联
	var existing []entity.CurrencyExchanges
	if err := c.db.WithContext(ctx).
		Where("exchange_id = ? AND currency_id IN ?", exchangeId, currencyIds).
		Find(&existing).Error; err != nil {
		return err
	}

	// 构建已存在币种ID集合
	existMap := make(map[int64]struct{})
	for _, e := range existing {
		existMap[e.CurrencyId] = struct{}{}
	}

	// 2. 构建待插入的关联记录
	var toInsert []entity.CurrencyExchanges
	for _, cid := range currencyIds {
		if _, ok := existMap[cid]; !ok {
			toInsert = append(toInsert, entity.CurrencyExchanges{
				CurrencyId: cid,
				ExchangeId: exchangeId,
				IsActive:   true,
			})
		}
	}

	// 3. 批量插入
	if len(toInsert) > 0 {
		if err := c.db.WithContext(ctx).Create(&toInsert).Error; err != nil {
			return err
		}
	}

	return nil
}
