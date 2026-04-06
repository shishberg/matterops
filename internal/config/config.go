package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
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

type Env struct {
	MattermostToken string
	WebhookSecret   string
}

func LoadEnv(path string) (*Env, error) {
	vars, err := godotenv.Read(path)
	if err != nil {
		return nil, fmt.Errorf("reading .env: %w", err)
	}

	return &Env{
		MattermostToken: vars["MATTERMOST_TOKEN"],
		WebhookSecret:   vars["GITHUB_WEBHOOK_SECRET"],
	}, nil
}

type ProcessConfig struct {
	Cmd string `yaml:"cmd"`
}

type ServiceConfig struct {
	Name                string        `yaml:"-"`
	Branch              string        `yaml:"branch"`
	Repo                string        `yaml:"repo"`
	WorkingDir          string        `yaml:"working_dir"`
	Deploy              []string      `yaml:"deploy"`
	Process             ProcessConfig `yaml:"process"`
	ServiceName         string        `yaml:"service_name"`
	RequireConfirmation bool          `yaml:"require_confirmation"`
}

func LoadServices(dir string) ([]ServiceConfig, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading services dir: %w", err)
	}

	var services []ServiceConfig
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		ext := filepath.Ext(entry.Name())
		if ext != ".yaml" && ext != ".yml" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading service file %s: %w", entry.Name(), err)
		}

		var svc ServiceConfig
		if err := yaml.Unmarshal(data, &svc); err != nil {
			return nil, fmt.Errorf("parsing service file %s: %w", entry.Name(), err)
		}

		if svc.Name == "" {
			svc.Name = strings.TrimSuffix(entry.Name(), ext)
		}

		services = append(services, svc)
	}

	return services, nil
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
