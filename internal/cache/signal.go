package cache

//import (
//	"context"
//	"database/sql"
//	"edgeflow/internal/dao"
//	"edgeflow/internal/model"
//	"sync"
//	"time"
//)
//
//// 缓存状态（信号快照）
//// cache.Latest["BTCUSDT"][1] // 最近的1级信号
//// cache.Latest["BTCUSDT"][2] // 最近的2级信号
//type signalCache struct {
//	Latest                 map[string]map[int]model.Signal // symbol → level → latest signal
//	Level3Buffer           []model.Signal
//	Level3UpgradeThreshold int // 触发升级的最小数量
//	mu                     sync.RWMutex
//	dao                    *dao.OrderDao
//}
//
//// NewSignalCache 创建缓存实例
//func NewSignalCache(d *dao.OrderDao) *signalCache {
//	return &signalCache{
//		Latest: make(map[string]map[int]model.Signal),
//		dao:    d,
//	}
//}
//
//// Get 获取信号（先查内存，再查数据库）
//func (c *signalCache) Get(symbol string, level int) (model.Signal, bool, error) {
//	c.mu.RLock()
//	if levelMap, ok := c.Latest[symbol]; ok {
//		if sig, exists := levelMap[level]; exists {
//			c.mu.RUnlock()
//			return sig, true, nil // 命中缓存
//		}
//	}
//	c.mu.RUnlock()
//
//	// 没有命中缓存，从数据库读取
//	sig, found, err := c.loadFromDB(symbol, level)
//	if err != nil || !found {
//		return model.Signal{}, false, err
//	}
//
//	// 存到缓存
//	c.mu.Lock()
//	if _, ok := c.Latest[symbol]; !ok {
//		c.Latest[symbol] = make(map[int]Signal)
//	}
//	c.Latest[symbol][level] = sig
//	c.mu.Unlock()
//
//	return sig, true, nil
//}
//
//// Set 更新缓存和数据库
//func (c *signalCache) Set(sig model.Signal) error {
//	// 更新缓存
//	c.mu.Lock()
//	if _, ok := c.Latest[sig.Symbol]; !ok {
//		c.Latest[sig.Symbol] = make(map[int]model.Signal)
//	}
//	c.Latest[sig.Symbol][sig.Level] = sig
//	c.mu.Unlock()
//
//	// 更新数据库
//	return c.saveToDB(sig)
//}
//
//// 从数据库读取
//func (c *signalCache) loadFromDB(symbol string, level int, td string) (model.OrderRecord, bool, error) {
//	sig, err := c.dao.OrderGetByLevel(context.Background(), symbol, level, td)
//
//	if err != nil {
//		return model.OrderRecord{}, false, err
//	}
//	return sig, true, nil
//}
//
//// 保存到数据库
//func (c *signalCache) saveToDB(sig model.Signal) error {
//
//	record := &model.OrderRecord{
//		OrderId:   sig.id,
//		Symbol:    order.Symbol,
//		CreatedAt: time.Time{},
//		Side:      order.Side,
//		Price:     order.Price,
//		Quantity:  order.Quantity,
//		OrderType: order.OrderType,
//		TP:        order.TPPrice,
//		SL:        order.SLPrice,
//		Strategy:  order.Strategy,
//		Comment:   order.Comment,
//		TradeType: order.TradeType,
//		MgnMode:   order.MgnMode,
//		Timestamp: order.Timestamp,
//		Level:     order.Level,
//		Score:     order.Score,
//	}
//	return c.dao.OrderCreateNew(context.Background(), record)
//
//}
