package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"time"
)

// 配置加载（API密钥等）

type WebhookConfig struct {
	Secret string `yaml:"secret"`
}

type Okx struct {
	ApiKey    string `yaml:"apiKey"`
	SecretKey string `yaml:"secretKey"`
	Password  string `yaml:"password"`
}

type Db struct {
	DbName   string `yaml:"dbname"`
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type StrategyConfig struct {
	// ---- Level 2 ----
	MinSpacingL2              time.Duration `yaml:"MinSpacingL2"`              // L2 信号间隔防抖
	RequireL1ConfirmForL2Open bool          `yaml:"RequireL1ConfirmForL2Open"` // 是否要求 L1 确认
	L1ConfirmMaxDelay         time.Duration `yaml:"L1ConfirmMaxDelay"`         // L1 确认最大延迟
	RequireTrendFilter        bool          `yaml:"RequireTrendFilter"`        // 是否启用趋势过滤

	// ---- Level 3 ----
	MinSpacingL3        time.Duration `yaml:"MinSpacingL3"`        // L3 信号间隔防抖
	CooldownAfterL2Flip time.Duration `yaml:"CooldownAfterL2Flip"` // L2 翻仓后的冷静期
	L3ReduceAtRMultiple float64       `yaml:"L3ReduceAtRMultiple"` // 浮盈多少倍R触发减仓
	L3ReducePercent     float64       `yaml:"L3ReducePercent"`     // 减仓比例 (0~1)

	// ---- 日志 / 调试 ----
	EnableDebugLog bool `yaml:"EnableDebugLog"` // 是否打印 Debug 日志
}

type Config struct {
	Webhook   WebhookConfig `yaml:"webhook"`
	Okx       `yaml:"okx"`
	Db        `yaml:"database"`
	Simulated bool           `yaml:"simulated"`
	Strategy  StrategyConfig `yaml:"strategy"`
}

var AppConfig Config

func LoadConfig(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("Read config file error %w", err)
	}
	if err := yaml.Unmarshal(data, &AppConfig); err != nil {
		return fmt.Errorf("Unmarshal config yaml error: %w", err)
	}
	return nil
}
