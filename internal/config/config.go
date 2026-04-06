package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type MattermostConfig struct {
	URL     string `yaml:"url"`
	Channel string `yaml:"channel"`
}

type WebhookConfig struct {
	Port int `yaml:"port"`
}

type DashboardConfig struct {
	Port int `yaml:"port"`
}

type Config struct {
	Mattermost  MattermostConfig `yaml:"mattermost"`
	Webhook     WebhookConfig    `yaml:"webhook"`
	Dashboard   DashboardConfig  `yaml:"dashboard"`
	ServicesDir string           `yaml:"services_dir"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	cfg := &Config{
		Webhook:     WebhookConfig{Port: 8080},
		Dashboard:   DashboardConfig{Port: 8081},
		ServicesDir: "./services",
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	return cfg, nil
}
