package service

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"strconv"
)

var _ CoinService = (*coinService)(nil)

type CoinService interface {
	CoinCreateNew(ctx context.Context, coin, name, nameEn string, categoryId int64, priceScale int, logoUrl string) (res model.CoinCreateNewRes, err error)
	CoinListGetByCategory(ctx context.Context, categoryId int64) (model.CoinListRes, error)
}

type coinService struct {
	d dao.CoinDao
}

func NewCoinService(d dao.CoinDao) *coinService {
	return &coinService{d: d}
}

func (c *coinService) CoinCreateNew(ctx context.Context, coin, name, nameEn string, categoryId int64, priceScale int, logoUrl string) (res model.CoinCreateNewRes, err error) {
	co := entity.Coin{}
	co.Coin = coin
	co.Name = name
	co.NameEn = nameEn
	co.Status = 1
	co.Source = "OKX"
	co.CategoryID = categoryId
	co.PriceScale = priceScale
	co.LogoUrl = logoUrl

	err = c.d.CoinCreateNew(ctx, &co)
	if err != nil {
		return
	}

	res.CoinId = strconv.FormatInt(co.Id, 10)
	res.CategoryId = strconv.FormatInt(co.CategoryID, 10)
	res.NameEN = co.NameEn
	res.Name = co.Name
	res.LogoUrl = co.LogoUrl

	return

}

func (c *coinService) CoinListGetByCategory(ctx context.Context, categoryId int64) (res model.CoinListRes, err error) {

	coinList, err := c.d.CoinGetListByCategory(ctx, categoryId)
	if err != nil {
		return
	}

	var temp model.CoinOneRes
	var listRes []model.CoinOneRes
	for _, v := range coinList {
		temp.CategoryId = strconv.FormatInt(v.CategoryId, 10)
		temp.CoinId = strconv.FormatInt(v.CoinId, 10)
		temp.LogoUrl = v.LogoUrl
		temp.NameEN = v.NameEN
		temp.Name = v.Name
		listRes = append(listRes, temp)
	}

	res.CoinList = listRes
	return
}
