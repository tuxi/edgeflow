package account

type Account struct {
	Currency  string  // 如 "USDT"
	Total     float64 // 总资产
	Available float64 // 可用资产
	Frozen    float64 // 冻结资产（如挂单锁定的部分）
}
