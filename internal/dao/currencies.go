package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type CurrenciesDao interface {
	// 创建货币
	CurrencyCreateNew(ctx context.Context, coin *entity.Currency) error
	// 创建货币 并关联到交易所
	CurrencyCreateNewWithExchange(ctx context.Context, exchangeId int64, currency *entity.Currency) error
	// 批量创建货币 并关联到交易所
	CurrencyCreateBatchWithExchange(ctx context.Context, exchangeId int64, currencies []entity.Currency) error
	// 更新币种
	CurrencyUpdate(ctx context.Context, coin *entity.Currency) error
	// 根据id获取coin
	CurrencyGetById(ctx context.Context, coinId int64) (model.CurrencyOne, error)
	// 根据coin标识获取coin
	CurrencyGetByCcy(ctx context.Context, coin string) (model.CurrencyOne, error)
	// 获取分类id获取coin列表
	CurrencyGetListByExchange(ctx context.Context, exId int64, page, limit int) (total int64, list []model.CurrencyOne, err error)

	// 创建交易所
	ExchangeCreateNew(ctx context.Context, name, nameEn string) (*model.Exchange, error)
	// 获取所有交易所
	ExchangesGet(ctx context.Context) ([]model.Exchange, error)

	// 关联交易所
	AssociateCurrencyWithExchange(ctx context.Context, currencyId, exchangeId int64) error
	// 批量关联交易所
	AssociateCurrenciesWithExchangeBatch(ctx context.Context, currencyIds []int64, exchangeId int64) error
}
