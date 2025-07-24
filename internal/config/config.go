package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
)

// 配置加载（API密钥等）

type WebhookConfig struct {
	Secret string `yaml:"secret"`
}

type Config struct {
	Webhook WebhookConfig `yaml:"webhook"`
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
