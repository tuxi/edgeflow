package model

// PositionState 模拟订单管理系统 (OMS) 提供的当前仓位信息
type PositionState struct {
	Symbol      string
	HasLong     bool    // 是否持有该币种的多仓
	HasShort    bool    // 是否持有该币种的空仓
	LongVolume  float64 // 多仓持仓量 (如果需要)
	ShortVolume float64 // 空仓持仓量 (如果需要)
	// Price, Cost, Margin... (后续扩展)
}
