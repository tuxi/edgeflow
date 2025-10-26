package service

import (
	"context"
	"edgeflow/internal/dao"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"edgeflow/pkg/exchange/okx"
	"log"
	"strconv"
	"time"
)

var _ InstrumentService = (*instrumentService)(nil)

type InstrumentService interface {
	StartInstrumentSyncWorker(ctx context.Context)

	InstrumentsCreateBatchWithExchange(ctx context.Context, exId int64, currencies []entity.CryptoInstrument) (err error)
	InstrumentsListGetByExchange(ctx context.Context, exId int64, page, limit int) (res model.CurrencyListRes, err error)
	InstrumentsGetAllByExchange(ctx context.Context, exId int64) (res model.CurrencyListRes, err error)
	ExchangeCreateNew(ctx context.Context, code, name string) (*model.Exchange, error)
}

type instrumentService struct {
	dao                    dao.CurrenciesDao
	syncInstrumentsHanlder func()
}

func NewInstrumentService(dao dao.CurrenciesDao, syncInstrumentsHanlder func()) InstrumentService {
	return &instrumentService{dao: dao, syncInstrumentsHanlder: syncInstrumentsHanlder}
}

// 定义同步间隔和交易所代码
const syncInterval = 1 * time.Hour

// 开启一个 Go Routine，定时同步 OKX 交易对元数据。
// 它会一直运行，直到传入的 Context 被取消。
func (s *instrumentService) StartInstrumentSyncWorker(ctx context.Context) {

	// 初始化交易所
	ex, err := s.ExchangeCreateNew(ctx, "OKX", "欧易OKX")
	if err != nil {
		panic(err)
	}

	// 立即开启一个 Go Routine，将同步逻辑与主程序解耦
	go func() {
		log.Printf("INFO: [%s] Instrument Sync Worker started. Initial sync imminent, then every: %v", ex.Code, syncInterval)

		// 1. 创建 Ticker，设置同步间隔
		ticker := time.NewTicker(syncInterval)
		defer ticker.Stop() // 确保 Go Routine 退出时 Ticker 被停止

		// 2. 立即执行一次初始同步，避免等待第一个间隔
		s.doSyncJob(ctx, *ex)

		// 3. 循环等待 Ticker 或 Context 取消信号
		for {
			select {
			case <-ctx.Done():
				// 接收到主程序的取消信号，优雅退出
				log.Printf("INFO: [%s] Instrument Sync Worker shutting down gracefully.", ex.Code)
				return

			case <-ticker.C:
				// 接收到 Ticker 信号，执行定时同步任务
				log.Printf("INFO: [%s] Ticker triggered. Starting instrument sync job.", ex.Code)
				s.doSyncJob(ctx, *ex)
			}
		}
	}()
}

// doSyncJob 执行实际的获取、筛选和 Upsert 逻辑
func (s *instrumentService) doSyncJob(ctx context.Context, exchange model.Exchange) {

	client := okx.NewPublicClient()
	// 1. 获取交易对列表（含重试逻辑）
	// 假设 GetInstrumentsWithRetry 封装了 3 次重试的逻辑
	rawList, err := client.GetInstrumentsWithRetry(ctx, "SPOT")
	if err != nil {
		log.Printf("FATAL: Failed to fetch instruments from %s after all retries: %v", exchange.Code, err)
		return // 等待下一个同步周期
	}

	// 2. 筛选 USDT 交易对 (业务需求)
	// 假设在服务层进行筛选和数据转换，以保持此处的业务简洁

	// 3. 调用服务层，执行数据转换和批量 Upsert
	// SyncInstruments 内部会负责：
	// a) 筛选 USDT 计价币。
	// b) 将 OkxInstrumentRaw 转换为 entity.CryptoInstrument。
	// c) 调用 DAO 层的 InstrumentUpsertBatchWithExchange 进行原子性创建/更新（检查新币上市）。
	if err := s.SyncInstruments(ctx, uint(exchange.ExId), rawList); err != nil {
		log.Printf("ERROR: Failed to sync %s instruments to DB: %v", exchange.Code, err)
		return // 等待下一个同步周期
	}

	log.Printf("INFO: Successfully processed and synced %d instruments for %s.", len(rawList), exchange.Code)
}

// 核心接口：负责将外部的原始数据列表转换为内部模型，并进行批量 Upsert
// exchangeCode: 交易所代码 (如 "OKX")
// rawInstruments: 从 OKX API 拉取的原始交易对数据列表 (可能是 []byte 或 []struct)
func (s *instrumentService) SyncInstruments(ctx context.Context, exchangeId uint, rawInstruments []okx.InstrumentRaw) error {
	var list []entity.CryptoInstrument
	for _, item := range rawInstruments {
		if item.QuoteCcy != "USDT" {
			continue
		}
		co, err := convertOKXToInstrumentEntity(&item, exchangeId)
		if err != nil {
			continue
		}
		list = append(list, *co)
	}

	err := s.dao.InstrumentUpsertBatchWithExchange(ctx, list)
	if err != nil && s.syncInstrumentsHanlder != nil {
		s.syncInstrumentsHanlder() // 通知同步完成
	}
	return err
}

// ConvertOKXToInstrumentEntity 将 OKX 原始数据转换为内部实体
// exchangeID: 目标交易所的数据库 ID
func convertOKXToInstrumentEntity(
	okxInstrument *okx.InstrumentRaw,
	exchangeID uint) (*entity.CryptoInstrument, error) {

	// 1. 数据校验与清洗
	if okxInstrument.InstType != "SPOT" {
		// 我们只同步现货交易对作为元数据基础
		return nil, nil
	}

	// 2. 状态映射
	var status string
	if okxInstrument.State == "live" {
		status = "LIVE"
	} else if okxInstrument.State == "suspend" {
		status = "SUSPENDED"
	} else {
		status = "DELISTED" // 其他状态统一归为 DELISTED
	}

	// 3. 数量精度选择
	// OKX 现货交易对，最小下单量通常是 minSz 或 lotSz，我们选择 minSz 作为数量精度
	qtyPrecision := okxInstrument.MinSz

	// 4. 转换并创建实体
	now := time.Now()
	instrument := &entity.CryptoInstrument{
		// 核心联合主键字段
		ExchangeID:   exchangeID,
		InstrumentID: okxInstrument.InstId,

		// 基础信息
		BaseCcy:    okxInstrument.BaseCcy,
		QuoteCcy:   okxInstrument.QuoteCcy,
		Status:     status,
		IsContract: false, // 默认现货不是合约标的 (除非您有独立的合约元数据表)

		// 精度 (存储为 string 以保持精确度)
		PricePrecision: okxInstrument.TickSz,
		QtyPrecision:   qtyPrecision,

		// 业务信息 (需要在后续流程中补充，初始化时为空)
		NameCN:    okxInstrument.BaseCcy, // 初始使用 ccy 占位，等待外部 API 补充
		NameEN:    okxInstrument.BaseCcy, // 初始使用 ccy 占位，等待外部 API 补充
		MarketCap: 0,

		// 时间戳
		CreatedAt: now,
		UpdatedAt: now,
		// Tags: 需要另行处理多对多关系
	}

	// 补充：如果需要初始化 listTime，可以这样转换：
	if okxInstrument.ListTime != "" {
		if ms, err := strconv.ParseInt(okxInstrument.ListTime, 10, 64); err == nil {
			// 将毫秒转为 time.Time
			instrument.CreatedAt = time.Unix(ms/1000, (ms%1000)*int64(time.Millisecond))
		}
	}

	return instrument, nil
}

func (c *instrumentService) InstrumentsCreateBatchWithExchange(ctx context.Context, exId int64, currencies []entity.CryptoInstrument) (err error) {

	err = c.dao.InstrumentUpsertBatchWithExchange(ctx, currencies)

	return
}

func (c *instrumentService) InstrumentsListGetByExchange(ctx context.Context, exId int64, page, limit int) (res model.CurrencyListRes, err error) {

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

func (c *instrumentService) ExchangeCreateNew(ctx context.Context, code, name string) (*model.Exchange, error) {

	ex, err := c.dao.ExchangeCreateNew(ctx, code, name)
	return ex, err
}

func (c *instrumentService) InstrumentsGetAllByExchange(ctx context.Context, exId int64) (res model.CurrencyListRes, err error) {
	list, err := c.dao.InstrumentsGetListByExchange(ctx, uint(exId), "USDT")
	if err != nil {
		return
	}
	res.List = list
	res.Total = int64(len(list))
	return res, err
}
