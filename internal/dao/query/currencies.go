package query

import (
	"context"
	"edgeflow/internal/model"
	"edgeflow/internal/model/entity"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"time"
)

type currenciesDao struct {
	db *gorm.DB
}

func NewCurrenciesDao(db *gorm.DB) *currenciesDao {
	return &currenciesDao{db: db}
}

func (c *currenciesDao) CurrencyCreateNew(ctx context.Context, coin *entity.CryptoInstrument) error {
	if coin == nil {
		return gorm.ErrInvalidData
	}
	// 手动设置时间戳，确保 NOT NULL 字段完整性
	now := time.Now()
	coin.CreatedAt = now
	coin.UpdatedAt = now
	return c.db.WithContext(ctx).Create(coin).Error
}

// 修正后的 CurrencyUpsert
func (dao *currenciesDao) CurrencyUpsert(ctx context.Context, w *entity.CryptoInstrument) error {
	if w == nil {
		return gorm.ErrInvalidData
	}

	// 设置时间戳
	now := time.Now()
	w.UpdatedAt = now

	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// 使用联合唯一索引作为冲突判断依据
			Columns: []clause.Column{
				{Name: "exchange_id"},
				{Name: "instrument_id"},
			},
			// 确保更新所有字段，但 GORM 会自动排除主键字段
			UpdateAll: true,
		}).
		Create(w).Error
}

// 批量 Upsert Whale
func (dao *currenciesDao) CurrencyUpsertBatch(ctx context.Context, currencies []*entity.CryptoInstrument) error {
	if len(currencies) == 0 {
		return nil
	}
	return dao.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// 使用联合唯一索引作为冲突判断依据
			Columns: []clause.Column{
				{Name: "exchange_id"},
				{Name: "instrument_id"},
			},
			UpdateAll: true,
		}).
		CreateInBatches(currencies, 100).Error
}

func (c *currenciesDao) CurrencyUpdate(ctx context.Context, coin *entity.CryptoInstrument) error {
	if coin == nil {
		return gorm.ErrInvalidData
	}
	coin.UpdatedAt = time.Now()
	// 推荐：如果使用结构体更新，最好指定哪些字段需要被更新
	// 示例：只更新价格精度和状态字段
	// return c.db.WithContext(ctx).Model(coin).Select("Status", "PricePrecision", "UpdatedAt").Updates(coin).Error

	// 如果 Updates(coin) 没有指定 Model，GORM 会使用主键进行更新
	return c.db.WithContext(ctx).Updates(coin).Error
}

func (c *currenciesDao) CurrencyGetById(ctx context.Context, coinId int64) (res model.CurrencyOne, err error) {
	err = c.db.WithContext(ctx).Where("id = ?", coinId).Find(&res).Error
	return
}

func (c *currenciesDao) CurrencyGetByCcy(ctx context.Context, baseCcy string) (res model.CurrencyOne, err error) {
	err = c.db.WithContext(ctx).Where("base_ccy = ?", baseCcy).Find(&res).Error
	return
}

func (c *currenciesDao) CurrencyGetListByExchange(ctx context.Context, exId uint, page, limit int) (total int64, list []entity.CryptoInstrument, err error) {
	if limit <= 0 {
		limit = 100 // 限制默认值
	}
	offset := (page - 1) * limit

	// 使用 GORM Model 构建查询的基础作用域 (Scope)
	tx := c.db.WithContext(ctx).
		// 直接操作我们设计的交易对元数据表
		Model(&entity.CryptoInstrument{}).
		// 筛选条件直接基于 exchange_id
		Where("exchange_id = ? AND status = ?", exId, "LIVE") // 假设 'LIVE' 是激活状态

	// 统计总数 (使用 Count)
	if err = tx.Count(&total).Error; err != nil {
		return
	}

	//  查询分页数据 (使用 Find)
	// 假设您希望按市值降序排列
	if err = tx.
		Order("market_cap DESC").
		Limit(limit).
		Offset(offset).
		// 使用 Find 将结果直接映射到结构体列表
		Find(&list).Error; err != nil {
		return
	}

	return total, list, nil
}

// 创建一个交易对并关联到交易所
func (c *currenciesDao) InstrumentUpsertWithExchange(ctx context.Context, instrument *entity.CryptoInstrument) error {

	if instrument == nil {
		return gorm.ErrInvalidData
	}

	// 1. 自动设置/更新时间戳
	now := time.Now()
	instrument.UpdatedAt = now
	if instrument.CreatedAt.IsZero() {
		instrument.CreatedAt = now
	}

	// 2. 使用 OnConflict 子句进行 Upsert 操作
	// 冲突判断依据：(exchange_id, instrument_id) 联合唯一索引
	return c.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// 明确指定联合唯一索引作为冲突判断的列
			Columns: []clause.Column{
				{Name: "exchange_id"},
				{Name: "instrument_id"},
			},
			// 发生冲突时，更新所有非主键字段
			// GORM 会自动处理更新操作，确保最新的数据写入
			UpdateAll: true,
		}).
		Create(instrument).Error
}

// 批量创建货币，并关联到某个交易所
func (c *currenciesDao) InstrumentUpsertBatchWithExchange(ctx context.Context, instruments []entity.CryptoInstrument) error {

	if len(instruments) == 0 {
		return nil
	}

	// 1. 预处理：批量设置时间戳和交易所ID
	// 注意：假设 instruments 列表在传入前已被服务层填充了正确的 ExchangeID
	now := time.Now()
	for _, instr := range instruments {
		// 更新 UpdatedAt 字段
		instr.UpdatedAt = now
		// 如果是新记录，设置 CreatedAt
		if instr.CreatedAt.IsZero() {
			instr.CreatedAt = now
		}
	}

	// 2. 使用 OnConflict + CreateInBatches 实现批量 Upsert
	// Upsert 是一种原子操作，无需额外的事务包裹。
	return c.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			// 冲突判断依据：(exchange_id, instrument_id) 联合唯一索引
			Columns: []clause.Column{
				{Name: "exchange_id"},
				{Name: "instrument_id"},
			},
			// 发生冲突时，更新所有非主键字段
			UpdateAll: true,
		}).
		// 使用 CreateInBatches 进行批量插入/更新，提高数据库操作效率
		// 批次大小 100 是一个常见的优化值
		CreateInBatches(instruments, 100).Error
}

func (c *currenciesDao) ExchangeCreateNew(ctx context.Context, code, name string) (*model.Exchange, error) {
	// 准备要创建的数据对象
	// 使用大写状态和 time.Now() 确保数据完整性
	ex := entity.CryptoExchange{
		Code:      code,
		Name:      name,
		Status:    "ACTIVE",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// GORM 的 FirstOrCreate 方法：
	// 1. 先根据 Where 条件 (Code = ?) 查询记录。
	// 2. 如果找到，则将结果填充到 ex，并返回 nil error。
	// 3. 如果未找到，则使用 ex 对象的其余字段值执行创建操作。
	result := c.db.WithContext(ctx).Where("code = ?", code).FirstOrCreate(&ex)

	if result.Error != nil {
		// 如果创建或查询过程中发生错误（如数据库连接失败、唯一性约束冲突等）
		return nil, result.Error
	}

	// 成功找到或创建
	return &model.Exchange{
		ExId: int64(ex.ID),
		Code: ex.Code,
		Name: ex.Name,
	}, nil
}

func (c *currenciesDao) ExchangesGet(ctx context.Context) ([]model.Exchange, error) {
	var exs []model.Exchange
	err := c.db.WithContext(ctx).Find(&exs).Error
	if err != nil {
		return nil, err
	}

	return exs, nil
}
