package service

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type InstrumentService struct {
	dao dao.CurrenciesDao
}

func NewInstrumentService(dao dao.CurrenciesDao) *InstrumentService {
	return &InstrumentService{dao: dao}
}

func (c *InstrumentService) InstrumentsCreateBatchWithExchange(ctx context.Context, exId int64, currencies []entity.CryptoInstrument) (err error) {

	err = c.dao.InstrumentUpsertBatchWithExchange(ctx, currencies)

	return
}

func (c *InstrumentService) InstrumentsListGetByExchange(ctx context.Context, exId int64, page, limit int) (res model.CurrencyListRes, err error) {

	total, coinList, err := c.dao.CurrencyGetListByExchange(ctx, uint(exId), page, limit)
	if err != nil {
		return
	}

	//var temp model.CurrencyOneRes
	//var listRes []model.CurrencyOneRes
	//for _, v := range coinList {
	//	temp.ID = strconv.FormatInt(int64(v.ID), 10)
	//	temp.ExchangeID = strconv.FormatInt(int64(v.ExchangeID), 10)
	//	temp.BaseCcy = v.BaseCcy
	//	temp.QuoteCcy = v.QuoteCcy
	//	temp.NameEN = v.NameEN
	//	temp.NameCN = v.NameCN
	//	temp.InstrumentID = v.InstrumentID
	//	temp.PricePrecision = v.PricePrecision
	//	temp.QtyPrecision = v.QtyPrecision
	//	listRes = append(listRes, temp)
	//}
	res.List = coinList
	res.Total = total
	return
}

func (c *InstrumentService) ExchangeCreateNew(ctx context.Context, code, name string) (*model.Exchange, error) {

	ex, err := c.dao.ExchangeCreateNew(ctx, code, name)
	return ex, err
}

func (c *InstrumentService) InstrumentsGetAllByExchange(ctx context.Context, exId int64) (res model.CurrencyListRes, err error) {
	list, err := c.dao.InstrumentsGetListByExchange(ctx, uint(exId), "USDT")
	if err != nil {
		return
	}
	res.List = list
	res.Total = int64(len(list))
	return res, err
}
