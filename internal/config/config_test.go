package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dmcleish91/matterops/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
mattermost:
  url: "https://mm.example.com"
  channel: "ops"
webhook:
  port: 9090
dashboard:
  port: 9091
services_dir: "./services"
`), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadConfig(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, "https://mm.example.com", cfg.Mattermost.URL)
	assert.Equal(t, "ops", cfg.Mattermost.Channel)
	assert.Equal(t, 9090, cfg.Webhook.Port)
	assert.Equal(t, 9091, cfg.Dashboard.Port)
	assert.Equal(t, "./services", cfg.ServicesDir)
}

func TestLoadConfigDefaults(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(cfgPath, []byte(`
mattermost:
  url: "https://mm.example.com"
  channel: "ops"
`), 0644)
	require.NoError(t, err)

	cfg, err := config.LoadConfig(cfgPath)
	require.NoError(t, err)

	assert.Equal(t, 8080, cfg.Webhook.Port)
	assert.Equal(t, 8081, cfg.Dashboard.Port)
	assert.Equal(t, "./services", cfg.ServicesDir)
}

func TestLoadEnv(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")
	err := os.WriteFile(envPath, []byte(`MATTERMOST_TOKEN=test-token-123
GITHUB_WEBHOOK_SECRET=whsec_test456
`), 0644)
	require.NoError(t, err)

	env, err := config.LoadEnv(envPath)
	require.NoError(t, err)

	assert.Equal(t, "test-token-123", env.MattermostToken)
	assert.Equal(t, "whsec_test456", env.WebhookSecret)
}

func TestLoadEnvMissing(t *testing.T) {
	_, err := config.LoadEnv("/nonexistent/.env")
	assert.Error(t, err)
}
