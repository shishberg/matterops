package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shishberg/matterops/internal/config"
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

func TestLoadServices(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "services")
	require.NoError(t, os.MkdirAll(svcDir, 0755))

	err := os.WriteFile(filepath.Join(svcDir, "myapp.yaml"), []byte(`
branch: main
repo: "github.com/org/myapp"
working_dir: "/opt/myapp"
deploy:
  - "git pull origin main"
  - "make build"
process:
  cmd: "./bin/myapp"
`), 0644)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(svcDir, "api.yaml"), []byte(`
branch: main
repo: "github.com/org/api"
working_dir: "/opt/api"
deploy:
  - "git pull origin main"
  - "go build -o bin/api ."
service_name: api
require_confirmation: true
`), 0644)
	require.NoError(t, err)

	services, err := config.LoadServices(svcDir)
	require.NoError(t, err)
	require.Len(t, services, 2)

	api := findService(services, "api")
	require.NotNil(t, api)
	assert.Equal(t, "api", api.Name)
	assert.Equal(t, "main", api.Branch)
	assert.Equal(t, "github.com/org/api", api.Repo)
	assert.Equal(t, "api", api.ServiceName)
	assert.True(t, api.RequireConfirmation)

	myapp := findService(services, "myapp")
	require.NotNil(t, myapp)
	assert.Equal(t, "myapp", myapp.Name)
	assert.Equal(t, "./bin/myapp", myapp.Process.Cmd)
	assert.False(t, myapp.RequireConfirmation)
}

func findService(services []config.ServiceConfig, name string) *config.ServiceConfig {
	for i := range services {
		if services[i].Name == name {
			return &services[i]
		}
	}
	return nil
}

func TestLoadServicesEmptyDir(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "services")
	require.NoError(t, os.MkdirAll(svcDir, 0755))

	services, err := config.LoadServices(svcDir)
	require.NoError(t, err)
	assert.Empty(t, services)
}

func TestLoadServicesIgnoresNonYaml(t *testing.T) {
	dir := t.TempDir()
	svcDir := filepath.Join(dir, "services")
	require.NoError(t, os.MkdirAll(svcDir, 0755))

	err := os.WriteFile(filepath.Join(svcDir, "README.md"), []byte("not a service"), 0644)
	require.NoError(t, err)

	services, err := config.LoadServices(svcDir)
	require.NoError(t, err)
	assert.Empty(t, services)
}
