package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shishberg/matterops/internal/app"
	"github.com/stretchr/testify/require"
)

func TestNewApp_LoadsConfig(t *testing.T) {
	dir := t.TempDir()

	// Write config.yaml
	cfgContent := `
mattermost:
  url: "http://localhost:8065"
  channel: "town-square"
webhook:
  port: 9080
dashboard:
  port: 9081
services_dir: "` + filepath.Join(dir, "services") + `"
`
	cfgPath := filepath.Join(dir, "config.yaml")
	require.NoError(t, os.WriteFile(cfgPath, []byte(cfgContent), 0600))

	// Write services directory with one test service
	servicesDir := filepath.Join(dir, "services")
	require.NoError(t, os.MkdirAll(servicesDir, 0750))

	svcContent := `
repo: "github.com/example/app"
branch: "main"
working_dir: "` + dir + `"
deploy:
  - "echo deployed"
process:
  cmd: "echo running"
`
	require.NoError(t, os.WriteFile(filepath.Join(servicesDir, "test.yaml"), []byte(svcContent), 0600))

	// Write .env
	envContent := "MATTERMOST_TOKEN=test-token\nGITHUB_WEBHOOK_SECRET=test-secret\n"
	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte(envContent), 0600))

	repoRoot := findRepoRoot(t)
	templatesFS := os.DirFS(filepath.Join(repoRoot, "templates"))

	a, err := app.New(cfgPath, envPath, templatesFS)
	require.NoError(t, err)
	defer a.Shutdown()
}

// findRepoRoot walks up from the test file's directory until it finds go.mod.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find repo root (go.mod)")
		}
		dir = parent
	}
}
