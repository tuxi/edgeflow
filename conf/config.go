package conf

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
	Simulated bool   `yaml:"simulated"`
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

type LogConfig struct {
	Level      string `yaml:"level"`
	FileName   string `yaml:"file-name"`
	TimeFormat string `yaml:"time-format"`
	MaxSize    int    `yaml:"max-size"`
	MaxBackups int    `yaml:"max-backups"`
	MaxAge     int    `yaml:"max-age"`
	Compress   bool   `yaml:"compress"`
	LocalTime  bool   `yaml:"local-time"`
	Console    bool   `yaml:"console"`
}

// RedisConfig is used to configure redis
type RedisConfig struct {
	Addr         string `yaml:"address"`
	Password     string `yaml:"password"`
	Db           int    `yaml:"db"`
	PoolSize     int    `yaml:"pool-size"`
	MinIdleConns int    `yaml:"min-idle-conns"`
	IdleTimeout  int    `yaml:"idle-timeout"`
}

type JwtConfig struct {
	Secret                  string `yaml:"secret"`
	JwtTtl                  int64  `yaml:"ttl"`              // token 有效期（秒）
	JwtBlacklistGracePeriod int64  `yaml:"blacklistperiod" ` // 黑名单宽限时间（秒）
}

type KafkaConfig struct {
	Broker string `yaml:"broker"`
}

type ProxyURL struct {
	Mode string `toml:"mode"` // 代理模式：sock5 或者http
	Ip   string `toml:"ip"`   // 代理的ip地址
	Port string `toml:"port"` // 代理的端口号
}

type EmailCofig struct {
	Host     string   `toml:"smtp_host"`
	Port     string   `toml:"smtp_port"`
	Username string   `toml:"smtp_user"`
	Password string   `toml:"smtp_password"`
	Sender   string   `toml:"smtp_sender"`
	ProxyURL ProxyURL `toml:"proxy_url"`
	PreCheck bool     `toml:"precheck"`
}

type InApps struct {
	Kid      string `toml:"kid"`      // 密钥ID
	Iss      string `toml:"iss"`      // Issuser ID 在用户和访问-》密钥中获取
	Bid      string `toml:"bid"`      // bundle id
	Password string `toml:"password"` // 共享密钥，在用户和访问-》共享密码 生成
	IsProd   bool   `toml:"is_prod"`
}

type AppleConfig struct {
	InApps InApps `toml:"in_apps"`
	Apns   Apns   `toml:"apns"`
}

type Apns struct {
	Topic          string `toml:"topic"`
	KeyID          string `toml:"key_id"`
	TeamID         string `toml:"team_id"`
	PayloadMaximum int    `toml:"payload_maximum"`
	IsProd         bool   `toml:"is_prod"`
}

type Config struct {
	AppName      string `yaml:"app_name"`
	Listen       string `yaml:"listen"`
	Mode         string `yaml:"mode"`
	Language     string `yaml:"language"`
	MaxPingCount int    `yaml:"max-ping-count"`
	ExternalURL  string `yaml:"external_url"`

	Webhook  WebhookConfig `yaml:"webhook"`
	Okx      `yaml:"okx"`
	Db       `yaml:"database"`
	Strategy StrategyConfig `yaml:"strategy"`
	Log      LogConfig      `yaml:"log"`
	Jwt      JwtConfig      `yaml:"jwt"`
	Redis    RedisConfig    `yaml:"redis"`
	Email    EmailCofig     `yaml:"email"`
	Apple    AppleConfig    `yaml:"apple"`
	Kafka    KafkaConfig    `yaml:"kafka"`
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
