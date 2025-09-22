package dao

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
)

type CoinDao interface {
	// 创建币种
	CoinCreateNew(ctx context.Context, coin *entity.Coin) error
	// 更新币种
	CoinUpdate(ctx context.Context, coin *entity.Coin) error
	// 根据id获取coin
	CoinGetById(ctx context.Context, coinId int64) (model.CoinOne, error)
	// 根据coin标识获取coin
	CoinGetByCoin(ctx context.Context, coin string) (model.CoinOne, error)
	// 获取分类id获取coin列表
	CoinGetListByCategory(ctx context.Context, categoryId int64, page, limit int) ([]model.CoinOne, error)
}
