package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type CurrenciesDao interface {
	CurrencyCreateNew(ctx context.Context, coin *entity.CryptoInstrument) error

	// 批量 Upsert Whale
	CurrencyUpsertBatch(ctx context.Context, currencies []*entity.CryptoInstrument) error

	CurrencyUpdate(ctx context.Context, coin *entity.CryptoInstrument) error

	CurrencyGetById(ctx context.Context, coinId int64) (res model.CurrencyOne, err error)

	CurrencyGetByCcy(ctx context.Context, baseCcy string) (res model.CurrencyOne, err error)

	CurrencyGetListByExchange(ctx context.Context, exId uint, page, limit int) (total int64, list []entity.CryptoInstrument, err error)

	// 获取全部交易对
	InstrumentsGetListByExchange(ctx context.Context, exId uint, quoteCcy string) (list []entity.CryptoInstrument, err error)

	// 创建一个交易对并关联到交易所
	InstrumentUpsertWithExchange(ctx context.Context, instrument *entity.CryptoInstrument) error

	// 批量创建货币，并关联到某个交易所
	InstrumentUpsertBatchWithExchange(ctx context.Context, instruments []entity.CryptoInstrument) error

	// 更新交易对状态：用于下架、上架
	UpdateInstrumentStatus(ctx context.Context, exID int64, instIDs []string, status string) error

	ExchangeCreateNew(ctx context.Context, code, name string) (*model.Exchange, error)

	ExchangesGet(ctx context.Context) ([]model.Exchange, error)
}
