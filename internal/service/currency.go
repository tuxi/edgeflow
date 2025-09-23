package service

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"strconv"
)

var _ CurrencyService = (*currencyService)(nil)

type CurrencyService interface {
	CurrencyCreateNew(ctx context.Context, ccy, name, nameEn string, priceScale int, logoUrl string) (res model.CurrencyCreateNewRes, err error)
	CurrencyCreateNewWithExchange(ctx context.Context, exId int64, ccy, name, nameEn string, priceScale int, logoUrl string) (res model.CurrencyCreateNewRes, err error)
	CurrencyCreateBatchWithExchange(ctx context.Context, exId int64, currencies []model.CurrencyOne) (err error)
	CurrencyListGetByExchange(ctx context.Context, exId int64, page, limit int) (model.CurrencyListRes, error)
	ExchangeCreateNew(ctx context.Context, name, nameEn string) error
	AssociateCurrencyWithExchange(ctx context.Context, currencyId, exchangeId int64) error
	AssociateCurrenciesWithExchangeBatch(ctx context.Context, currencyIds []int64, exchangeId int64) error
}

type currencyService struct {
	d dao.CurrenciesDao
}

func NewCoinService(d dao.CurrenciesDao) *currencyService {
	return &currencyService{d: d}
}

func (c *currencyService) CurrencyCreateNew(ctx context.Context, ccy, name, nameEn string, priceScale int, logoUrl string) (res model.CurrencyCreateNewRes, err error) {
	co := entity.Currency{}
	co.Ccy = ccy
	co.Name = name
	co.NameEn = nameEn
	co.Status = 1
	co.PriceScale = priceScale
	co.LogoUrl = logoUrl

	err = c.d.CurrencyCreateNew(ctx, &co)
	if err != nil {
		return
	}

	res.CcyId = strconv.FormatInt(co.Id, 10)
	res.NameEN = co.NameEn
	res.Name = co.Name
	res.Ccy = co.Ccy
	res.LogoUrl = co.LogoUrl

	return

}

func (c *currencyService) CurrencyCreateNewWithExchange(ctx context.Context, exId int64, ccy, name, nameEn string, priceScale int, logoUrl string) (res model.CurrencyCreateNewRes, err error) {
	co := entity.Currency{}
	co.Ccy = ccy
	co.Name = name
	co.NameEn = nameEn
	co.Status = 1
	co.PriceScale = priceScale
	co.LogoUrl = logoUrl

	err = c.d.CurrencyCreateNewWithExchange(ctx, exId, &co)

	if err != nil {
		return
	}

	res.CcyId = strconv.FormatInt(co.Id, 10)
	res.NameEN = co.NameEn
	res.Name = co.Name
	res.Ccy = co.Ccy
	res.LogoUrl = co.LogoUrl

	return
}

func (c *currencyService) CurrencyCreateBatchWithExchange(ctx context.Context, exId int64, currencies []model.CurrencyOne) (err error) {
	var list []entity.Currency
	for _, item := range currencies {
		var temp entity.Currency
		temp.Ccy = item.Ccy
		temp.Name = item.Name
		temp.NameEn = item.NameEN
		temp.Status = 1
		temp.PriceScale = item.PriceScale
		temp.LogoUrl = item.LogoUrl
	}

	err = c.d.CurrencyCreateBatchWithExchange(ctx, exId, list)

	return
}

func (c *currencyService) CurrencyListGetByExchange(ctx context.Context, exId int64, page, limit int) (res model.CurrencyListRes, err error) {

	total, coinList, err := c.d.CurrencyGetListByExchange(ctx, exId, page, limit)
	if err != nil {
		return
	}

	var temp model.CurrencyOneRes
	var listRes []model.CurrencyOneRes
	for _, v := range coinList {
		temp.CcyId = strconv.FormatInt(v.CcyId, 10)
		temp.LogoUrl = v.LogoUrl
		temp.NameEN = v.NameEN
		temp.Name = v.Name
		temp.Ccy = v.Ccy
		temp.PriceScale = v.PriceScale
		listRes = append(listRes, temp)
	}

	res.List = listRes
	res.Total = total
	return
}

func (c *currencyService) ExchangeCreateNew(ctx context.Context, name, nameEn string) error {

	_, err := c.d.ExchangeCreateNew(ctx, name, nameEn)
	return err
}

func (c *currencyService) AssociateCurrencyWithExchange(ctx context.Context, currencyId, exchangeId int64) error {
	return c.d.AssociateCurrencyWithExchange(ctx, currencyId, exchangeId)
}

func (c *currencyService) AssociateCurrenciesWithExchangeBatch(ctx context.Context, currencyIds []int64, exchangeId int64) error {
	return c.d.AssociateCurrenciesWithExchangeBatch(ctx, currencyIds, exchangeId)
}
