# MatterOps Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build a single Go binary that orchestrates service deployments, integrating with Mattermost for commands, GitHub webhooks for triggers, and a web dashboard for status.

**Architecture:** Monolithic single-binary with goroutines. Central `service.Manager` coordinates all actions. Three service backends (process, systemctl, launchctl) behind a common interface. Channel-based deploy queue per service.

**Tech Stack:** Go 1.22+, `gopkg.in/yaml.v3`, `github.com/joho/godotenv`, `github.com/stretchr/testify`, `github.com/mattermost/mattermost/server/public/model` (Mattermost client), `html/template`, `net/http`

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/matterops/main.go`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `.env.example`
- Create: `.golangci.yml`

- [ ] **Step 1: Initialize Go module**

```bash
cd /Users/agent/src/matterops
go mod init github.com/dmcleish91/matterops
```

- [ ] **Step 2: Create minimal main.go**

Create `cmd/matterops/main.go`:

```go
package main

import "fmt"

func main() {
	fmt.Println("matterops")
}
```

- [ ] **Step 3: Create Makefile**

Create `Makefile`:

```makefile
.PHONY: check lint test build dev playwright clean

check: lint test build ## Run all checks

lint: ## Run linter
	golangci-lint run ./...

test: ## Run tests
	go test -race -count=1 ./...

build: ## Build binary
	go build -o bin/matterops ./cmd/matterops

dev: ## Run in dev mode
	go run ./cmd/matterops --config config.dev.yaml

playwright: ## Run Playwright dashboard tests
	npx playwright test

clean: ## Clean build artifacts
	rm -rf bin/
```

- [ ] **Step 4: Create .gitignore**

Create `.gitignore`:

```
bin/
.env
*.exe
```

- [ ] **Step 5: Create .env.example**

Create `.env.example`:

```
MATTERMOST_TOKEN=
GITHUB_WEBHOOK_SECRET=
```

- [ ] **Step 6: Create .golangci.yml**

Create `.golangci.yml`:

```yaml
linters:
  enable:
    - gofmt
    - govet
    - errcheck
    - staticcheck
    - unused
    - gosimple
    - ineffassign
```

- [ ] **Step 7: Install golangci-lint if needed and verify**

```bash
which golangci-lint || go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
make check
```

Expected: Build succeeds, no lint errors, no tests yet.

- [ ] **Step 8: Commit**

```bash
git add -A
git commit -m "feat: scaffold project with go.mod, Makefile, and tooling config"
```

---

### Task 2: Config Package — Global Config

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for global config loading**

Create `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Install dependencies**

```bash
go get gopkg.in/yaml.v3
go get github.com/stretchr/testify
```

- [ ] **Step 4: Implement config loading**

Create `internal/config/config.go`:

```go
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
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/config/...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add config package with global config loading and defaults"
```

---

### Task 3: Config Package — .env Loading

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for .env loading**

Add to `internal/config/config_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/...
```

Expected: FAIL — `LoadEnv` not defined.

- [ ] **Step 3: Install godotenv and implement**

```bash
go get github.com/joho/godotenv
```

Add to `internal/config/config.go`:

```go
import (
	"github.com/joho/godotenv"
)

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
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add .env loading for secrets"
```

---

### Task 4: Config Package — Service Config Discovery

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing test for service config loading**

Add to `internal/config/config_test.go`:

```go
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

	// Sorted by name (filename)
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
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/config/...
```

Expected: FAIL — `LoadServices`, `ServiceConfig` not defined.

- [ ] **Step 3: Implement service config loading**

Add to `internal/config/config.go`:

```go
import (
	"path/filepath"
	"strings"
)

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
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/config/...
```

Expected: PASS

- [ ] **Step 5: Run make check**

```bash
make check
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/config/ go.mod go.sum
git commit -m "feat: add service config discovery from services/ directory"
```

---

### Task 5: Service Backend Interface + Process Backend

**Files:**
- Create: `internal/service/backend.go`
- Create: `internal/service/process.go`
- Create: `internal/service/process_test.go`

- [ ] **Step 1: Write failing test for process backend**

Create `internal/service/process_test.go`:

```go
package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessBackend_StartAndStatus(t *testing.T) {
	b := service.NewProcessBackend("sleep 60", t.TempDir())

	status, err := b.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "stopped", status)

	err = b.Start(context.Background())
	require.NoError(t, err)

	status, err = b.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "running", status)

	err = b.Stop(context.Background())
	require.NoError(t, err)

	// Give process time to exit
	time.Sleep(100 * time.Millisecond)

	status, err = b.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "stopped", status)
}

func TestProcessBackend_Restart(t *testing.T) {
	b := service.NewProcessBackend("sleep 60", t.TempDir())

	err := b.Start(context.Background())
	require.NoError(t, err)

	err = b.Restart(context.Background())
	require.NoError(t, err)

	status, err := b.Status(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "running", status)

	// Cleanup
	_ = b.Stop(context.Background())
}

func TestProcessBackend_StopWhenNotRunning(t *testing.T) {
	b := service.NewProcessBackend("sleep 60", t.TempDir())

	err := b.Stop(context.Background())
	assert.NoError(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Create backend interface**

Create `internal/service/backend.go`:

```go
package service

import "context"

// Backend manages the lifecycle of a service.
type Backend interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	Restart(ctx context.Context) error
	Status(ctx context.Context) (string, error)
}
```

- [ ] **Step 4: Implement process backend**

Create `internal/service/process.go`:

```go
package service

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
)

type ProcessBackend struct {
	cmd        string
	workingDir string
	mu         sync.Mutex
	process    *exec.Cmd
}

func NewProcessBackend(cmd string, workingDir string) *ProcessBackend {
	return &ProcessBackend{
		cmd:        cmd,
		workingDir: workingDir,
	}
}

func (p *ProcessBackend) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning() {
		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", p.cmd)
	cmd.Dir = p.workingDir
	// Set process group so we can kill all children
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	p.process = cmd

	// Reap the process in the background
	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

func (p *ProcessBackend) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning() {
		return nil
	}

	// Kill the process group
	pgid, err := syscall.Getpgid(p.process.Process.Pid)
	if err != nil {
		return err
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return err
	}

	p.process = nil
	return nil
}

func (p *ProcessBackend) Restart(ctx context.Context) error {
	if err := p.Stop(ctx); err != nil {
		return err
	}
	return p.Start(ctx)
}

func (p *ProcessBackend) Status(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning() {
		return "running", nil
	}
	return "stopped", nil
}

func (p *ProcessBackend) isRunning() bool {
	if p.process == nil || p.process.Process == nil {
		return false
	}
	// Check if process is still alive
	err := p.process.Process.Signal(syscall.Signal(0))
	return err == nil
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/service/...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/service/
git commit -m "feat: add service backend interface and process backend"
```

---

### Task 6: Deploy Runner

**Files:**
- Create: `internal/deploy/runner.go`
- Create: `internal/deploy/runner_test.go`

- [ ] **Step 1: Write failing test for deploy runner**

Create `internal/deploy/runner_test.go`:

```go
package deploy_test

import (
	"context"
	"testing"

	"github.com/dmcleish91/matterops/internal/deploy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunSteps_AllSucceed(t *testing.T) {
	steps := []string{
		"echo step1",
		"echo step2",
		"echo step3",
	}

	result, err := deploy.RunSteps(context.Background(), steps, t.TempDir())
	require.NoError(t, err)

	assert.Equal(t, "success", result.Status)
	assert.Contains(t, result.Output, "step1")
	assert.Contains(t, result.Output, "step2")
	assert.Contains(t, result.Output, "step3")
	assert.Empty(t, result.FailedStep)
}

func TestRunSteps_FailsOnStep(t *testing.T) {
	steps := []string{
		"echo ok",
		"exit 1",
		"echo should-not-run",
	}

	result, err := deploy.RunSteps(context.Background(), steps, t.TempDir())
	require.NoError(t, err)

	assert.Equal(t, "failed", result.Status)
	assert.Equal(t, "exit 1", result.FailedStep)
	assert.NotContains(t, result.Output, "should-not-run")
}

func TestRunSteps_CancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	steps := []string{"echo hello"}

	result, err := deploy.RunSteps(ctx, steps, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "failed", result.Status)
}

func TestRunSteps_EmptySteps(t *testing.T) {
	result, err := deploy.RunSteps(context.Background(), nil, t.TempDir())
	require.NoError(t, err)
	assert.Equal(t, "success", result.Status)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/deploy/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement deploy runner**

Create `internal/deploy/runner.go`:

```go
package deploy

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

type Result struct {
	Status     string // "success" or "failed"
	Output     string // combined stdout/stderr from all steps
	FailedStep string // which step failed, empty on success
}

func RunSteps(ctx context.Context, steps []string, workingDir string) (*Result, error) {
	var output bytes.Buffer

	for _, step := range steps {
		if ctx.Err() != nil {
			return &Result{
				Status:     "failed",
				Output:     output.String(),
				FailedStep: step,
			}, nil
		}

		fmt.Fprintf(&output, "$ %s\n", step)

		cmd := exec.CommandContext(ctx, "sh", "-c", step)
		cmd.Dir = workingDir
		cmd.Stdout = &output
		cmd.Stderr = &output

		if err := cmd.Run(); err != nil {
			fmt.Fprintf(&output, "ERROR: %s\n", err)
			return &Result{
				Status:     "failed",
				Output:     output.String(),
				FailedStep: step,
			}, nil
		}
	}

	return &Result{
		Status: "success",
		Output: output.String(),
	}, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/deploy/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/deploy/
git commit -m "feat: add deploy runner for executing ordered shell command steps"
```

---

### Task 7: Service Manager — State and Deploy Queue

**Files:**
- Create: `internal/service/manager.go`
- Create: `internal/service/manager_test.go`

- [ ] **Step 1: Write failing test for manager state tracking**

Create `internal/service/manager_test.go`:

```go
package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/dmcleish91/matterops/internal/config"
	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestManager(t *testing.T) *service.Manager {
	t.Helper()
	dir := t.TempDir()
	services := []config.ServiceConfig{
		{
			Name:       "testapp",
			Branch:     "main",
			Repo:       "github.com/org/testapp",
			WorkingDir: dir,
			Deploy:     []string{"echo deployed"},
			Process:    config.ProcessConfig{Cmd: "sleep 60"},
		},
	}
	m, err := service.NewManager(services, nil)
	require.NoError(t, err)
	return m
}

func TestManager_GetAllStates(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	states := m.GetAllStates()
	require.Len(t, states, 1)
	assert.Equal(t, "stopped", states["testapp"].Status)
}

func TestManager_GetState(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	state, ok := m.GetState("testapp")
	require.True(t, ok)
	assert.Equal(t, "stopped", state.Status)

	_, ok = m.GetState("nonexistent")
	assert.False(t, ok)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/...
```

Expected: FAIL — `Manager` not defined.

- [ ] **Step 3: Implement Manager with state tracking**

Create `internal/service/manager.go`:

```go
package service

import (
	"sync"
	"time"

	"github.com/dmcleish91/matterops/internal/config"
)

type ServiceState struct {
	Status     string    `json:"status"`      // "running", "stopped", "deploying", "failed"
	LastDeploy time.Time `json:"last_deploy"`
	LastResult string    `json:"last_result"` // "success", "failed"
	LastOutput string    `json:"last_output"`
	FailedStep string    `json:"failed_step"`
}

// Notifier receives deploy lifecycle events. Optional.
type Notifier interface {
	DeployStarted(serviceName string) error
	DeploySucceeded(serviceName string, output string) error
	DeployFailed(serviceName string, failedStep string, output string) error
	DeployQueued(serviceName string) error
}

type managedService struct {
	config  config.ServiceConfig
	backend Backend
	state   ServiceState
	queue   chan struct{} // capacity 1, latest-wins deploy queue
}

type Manager struct {
	mu       sync.RWMutex
	services map[string]*managedService
	notifier Notifier
	ctx      context.Context
	cancel   context.CancelFunc
}

func NewManager(configs []config.ServiceConfig, notifier Notifier) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		services: make(map[string]*managedService),
		notifier: notifier,
		ctx:      ctx,
		cancel:   cancel,
	}

	for _, cfg := range configs {
		backend := backendForConfig(cfg)
		ms := &managedService{
			config:  cfg,
			backend: backend,
			state:   ServiceState{Status: "stopped"},
			queue:   make(chan struct{}, 1),
		}
		m.services[cfg.Name] = ms
		go m.runWorker(ms)
	}

	return m, nil
}

func backendForConfig(cfg config.ServiceConfig) Backend {
	if cfg.Process.Cmd != "" {
		return NewProcessBackend(cfg.Process.Cmd, cfg.WorkingDir)
	}
	// TODO: systemctl/launchctl backends in later tasks
	return NewProcessBackend("echo no-op", cfg.WorkingDir)
}

func (m *Manager) Stop() {
	m.cancel()
}

func (m *Manager) GetAllStates() map[string]ServiceState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	states := make(map[string]ServiceState, len(m.services))
	for name, ms := range m.services {
		states[name] = ms.state
	}
	return states
}

func (m *Manager) GetState(name string) (ServiceState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ms, ok := m.services[name]
	if !ok {
		return ServiceState{}, false
	}
	return ms.state, true
}

func (m *Manager) GetServiceConfig(name string) (config.ServiceConfig, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	ms, ok := m.services[name]
	if !ok {
		return config.ServiceConfig{}, false
	}
	return ms.config, true
}

func (m *Manager) FindServiceByRepo(repo string, branch string) (string, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for name, ms := range m.services {
		if ms.config.Repo == repo && ms.config.Branch == branch {
			return name, true
		}
	}
	return "", false
}
```

Add the missing import at the top of `manager.go`:

```go
import (
	"context"
	"sync"
	"time"

	"github.com/dmcleish91/matterops/internal/config"
)
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/service/...
```

Expected: PASS (the new tests; `runWorker` can be a no-op for now).

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat: add service manager with state tracking"
```

---

### Task 8: Service Manager — Deploy Execution and Queue

**Files:**
- Modify: `internal/service/manager.go`
- Modify: `internal/service/manager_test.go`

- [ ] **Step 1: Write failing test for deploy execution**

Add to `internal/service/manager_test.go`:

```go
func TestManager_Deploy(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	err := m.RequestDeploy("testapp")
	require.NoError(t, err)

	// Wait for deploy to complete
	require.Eventually(t, func() bool {
		state, _ := m.GetState("testapp")
		return state.Status != "deploying"
	}, 5*time.Second, 50*time.Millisecond)

	state, _ := m.GetState("testapp")
	assert.Equal(t, "running", state.Status)
	assert.Equal(t, "success", state.LastResult)
	assert.Contains(t, state.LastOutput, "deployed")
	assert.False(t, state.LastDeploy.IsZero())
}

func TestManager_DeployUnknownService(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	err := m.RequestDeploy("nonexistent")
	assert.Error(t, err)
}

func TestManager_DeployQueueLatestWins(t *testing.T) {
	dir := t.TempDir()
	services := []config.ServiceConfig{
		{
			Name:       "slowapp",
			Branch:     "main",
			Repo:       "github.com/org/slowapp",
			WorkingDir: dir,
			Deploy:     []string{"sleep 1"},
			Process:    config.ProcessConfig{Cmd: "sleep 60"},
		},
	}
	m, err := service.NewManager(services, nil)
	require.NoError(t, err)
	defer m.Stop()

	// Trigger first deploy
	err = m.RequestDeploy("slowapp")
	require.NoError(t, err)

	// Wait until deploying
	require.Eventually(t, func() bool {
		state, _ := m.GetState("slowapp")
		return state.Status == "deploying"
	}, 2*time.Second, 50*time.Millisecond)

	// Queue two more deploys — only latest should survive
	err = m.RequestDeploy("slowapp")
	require.NoError(t, err)
	err = m.RequestDeploy("slowapp")
	require.NoError(t, err)

	// Wait for everything to settle
	require.Eventually(t, func() bool {
		state, _ := m.GetState("slowapp")
		return state.Status != "deploying"
	}, 10*time.Second, 100*time.Millisecond)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/...
```

Expected: FAIL — `RequestDeploy` not defined.

- [ ] **Step 3: Implement RequestDeploy and worker loop**

Add to `internal/service/manager.go`:

```go
import (
	"fmt"

	"github.com/dmcleish91/matterops/internal/deploy"
)

func (m *Manager) RequestDeploy(name string) error {
	m.mu.RLock()
	ms, ok := m.services[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown service: %s", name)
	}

	// Latest-wins: drain any pending request, then send a new one
	select {
	case <-ms.queue:
	default:
	}
	select {
	case ms.queue <- struct{}{}:
	default:
	}

	if m.notifier != nil {
		_ = m.notifier.DeployQueued(name)
	}

	return nil
}

func (m *Manager) runWorker(ms *managedService) {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ms.queue:
			m.executeDeploy(ms)
		}
	}
}

func (m *Manager) executeDeploy(ms *managedService) {
	name := ms.config.Name

	m.mu.Lock()
	ms.state.Status = "deploying"
	m.mu.Unlock()

	if m.notifier != nil {
		_ = m.notifier.DeployStarted(name)
	}

	result, err := deploy.RunSteps(m.ctx, ms.config.Deploy, ms.config.WorkingDir)
	if err != nil {
		m.mu.Lock()
		ms.state.Status = "failed"
		ms.state.LastDeploy = time.Now()
		ms.state.LastResult = "failed"
		ms.state.LastOutput = err.Error()
		m.mu.Unlock()

		if m.notifier != nil {
			_ = m.notifier.DeployFailed(name, "", err.Error())
		}
		return
	}

	m.mu.Lock()
	ms.state.LastDeploy = time.Now()
	ms.state.LastResult = result.Status
	ms.state.LastOutput = result.Output
	ms.state.FailedStep = result.FailedStep
	m.mu.Unlock()

	if result.Status == "failed" {
		m.mu.Lock()
		ms.state.Status = "failed"
		m.mu.Unlock()

		if m.notifier != nil {
			_ = m.notifier.DeployFailed(name, result.FailedStep, result.Output)
		}
		return
	}

	// Deploy succeeded — restart the service
	if err := ms.backend.Restart(m.ctx); err != nil {
		m.mu.Lock()
		ms.state.Status = "failed"
		m.mu.Unlock()

		if m.notifier != nil {
			_ = m.notifier.DeployFailed(name, "restart", err.Error())
		}
		return
	}

	m.mu.Lock()
	ms.state.Status = "running"
	m.mu.Unlock()

	if m.notifier != nil {
		_ = m.notifier.DeploySucceeded(name, result.Output)
	}
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./internal/service/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat: add deploy queue and execution to service manager"
```

---

### Task 9: Service Manager — Restart Command

**Files:**
- Modify: `internal/service/manager.go`
- Modify: `internal/service/manager_test.go`

- [ ] **Step 1: Write failing test for restart**

Add to `internal/service/manager_test.go`:

```go
func TestManager_Restart(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	err := m.RestartService("testapp")
	require.NoError(t, err)

	state, _ := m.GetState("testapp")
	assert.Equal(t, "running", state.Status)
}

func TestManager_RestartUnknown(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()

	err := m.RestartService("nonexistent")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/...
```

Expected: FAIL — `RestartService` not defined.

- [ ] **Step 3: Implement RestartService**

Add to `internal/service/manager.go`:

```go
func (m *Manager) RestartService(name string) error {
	m.mu.RLock()
	ms, ok := m.services[name]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown service: %s", name)
	}

	if err := ms.backend.Restart(m.ctx); err != nil {
		return fmt.Errorf("restarting %s: %w", name, err)
	}

	m.mu.Lock()
	ms.state.Status = "running"
	m.mu.Unlock()

	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test -race ./internal/service/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/
git commit -m "feat: add restart command to service manager"
```

---

### Task 10: GitHub Webhook Handler

**Files:**
- Create: `internal/webhook/handler.go`
- Create: `internal/webhook/handler_test.go`

- [ ] **Step 1: Write failing test for webhook signature validation and push parsing**

Create `internal/webhook/handler_test.go`:

```go
package webhook_test

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dmcleish91/matterops/internal/webhook"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func signPayload(secret string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type mockDeployTrigger struct {
	called     bool
	calledRepo string
	calledRef  string
}

func (m *mockDeployTrigger) HandlePush(repo string, branch string) {
	m.called = true
	m.calledRepo = repo
	m.calledRef = branch
}

func TestWebhook_ValidPush(t *testing.T) {
	mock := &mockDeployTrigger{}
	h := webhook.NewHandler("test-secret", mock)

	payload, _ := json.Marshal(map[string]interface{}{
		"ref": "refs/heads/main",
		"repository": map[string]interface{}{
			"full_name": "org/myapp",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", signPayload("test-secret", payload))
	req.Header.Set("X-GitHub-Event", "push")
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.True(t, mock.called)
	assert.Equal(t, "org/myapp", mock.calledRepo)
	assert.Equal(t, "main", mock.calledRef)
}

func TestWebhook_InvalidSignature(t *testing.T) {
	mock := &mockDeployTrigger{}
	h := webhook.NewHandler("test-secret", mock)

	payload := []byte(`{"ref":"refs/heads/main"}`)

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.False(t, mock.called)
}

func TestWebhook_NonPushEvent(t *testing.T) {
	mock := &mockDeployTrigger{}
	h := webhook.NewHandler("test-secret", mock)

	payload := []byte(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", signPayload("test-secret", payload))
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.False(t, mock.called)
}

func TestWebhook_TagPushIgnored(t *testing.T) {
	mock := &mockDeployTrigger{}
	h := webhook.NewHandler("test-secret", mock)

	payload, _ := json.Marshal(map[string]interface{}{
		"ref": "refs/tags/v1.0.0",
		"repository": map[string]interface{}{
			"full_name": "org/myapp",
		},
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook/github", strings.NewReader(string(payload)))
	req.Header.Set("X-Hub-Signature-256", signPayload("test-secret", payload))
	req.Header.Set("X-GitHub-Event", "push")

	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.False(t, mock.called)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/webhook/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement webhook handler**

Create `internal/webhook/handler.go`:

```go
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

// DeployTrigger is called when a valid push event is received.
type DeployTrigger interface {
	HandlePush(repo string, branch string)
}

type Handler struct {
	secret  string
	trigger DeployTrigger
}

func NewHandler(secret string, trigger DeployTrigger) *Handler {
	return &Handler{
		secret:  secret,
		trigger: trigger,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if !h.verifySignature(r.Header.Get("X-Hub-Signature-256"), body) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	event := r.Header.Get("X-GitHub-Event")
	if event != "push" {
		w.WriteHeader(http.StatusOK)
		return
	}

	var payload struct {
		Ref        string `json:"ref"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "bad payload", http.StatusBadRequest)
		return
	}

	// Only handle branch pushes, not tags
	if !strings.HasPrefix(payload.Ref, "refs/heads/") {
		w.WriteHeader(http.StatusOK)
		return
	}

	branch := strings.TrimPrefix(payload.Ref, "refs/heads/")
	h.trigger.HandlePush(payload.Repository.FullName, branch)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) verifySignature(signature string, body []byte) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	expected := hmac.New(sha256.New, []byte(h.secret))
	expected.Write(body)
	expectedSig := "sha256=" + hex.EncodeToString(expected.Sum(nil))

	return hmac.Equal([]byte(signature), []byte(expectedSig))
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/webhook/...
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/webhook/
git commit -m "feat: add GitHub webhook handler with signature validation"
```

---

### Task 11: Mattermost Bot — Connection and Message Parsing

**Files:**
- Create: `internal/bot/bot.go`
- Create: `internal/bot/bot_test.go`

- [ ] **Step 1: Write failing test for command parsing**

Create `internal/bot/bot_test.go`:

```go
package bot_test

import (
	"testing"

	"github.com/dmcleish91/matterops/internal/bot"
	"github.com/stretchr/testify/assert"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name    string
		message string
		want    *bot.Command
	}{
		{
			name:    "status command",
			message: "@matterops status",
			want:    &bot.Command{Action: "status"},
		},
		{
			name:    "deploy command",
			message: "@matterops deploy myapp",
			want:    &bot.Command{Action: "deploy", Service: "myapp"},
		},
		{
			name:    "restart command",
			message: "@matterops restart myapp",
			want:    &bot.Command{Action: "restart", Service: "myapp"},
		},
		{
			name:    "confirm command",
			message: "@matterops confirm myapp",
			want:    &bot.Command{Action: "confirm", Service: "myapp"},
		},
		{
			name:    "with extra whitespace",
			message: "  @matterops   deploy   myapp  ",
			want:    &bot.Command{Action: "deploy", Service: "myapp"},
		},
		{
			name:    "not a command",
			message: "hello world",
			want:    nil,
		},
		{
			name:    "empty after mention",
			message: "@matterops",
			want:    nil,
		},
		{
			name:    "unknown command",
			message: "@matterops foobar",
			want:    &bot.Command{Action: "foobar"},
		},
		{
			name:    "case insensitive mention",
			message: "@MatterOps deploy myapp",
			want:    &bot.Command{Action: "deploy", Service: "myapp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := bot.ParseCommand(tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/bot/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement command parsing and bot structure**

Create `internal/bot/bot.go`:

```go
package bot

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

type Command struct {
	Action  string
	Service string
}

func ParseCommand(message string) *Command {
	fields := strings.Fields(message)
	if len(fields) < 2 {
		return nil
	}

	if !strings.EqualFold(fields[0], "@matterops") {
		return nil
	}

	cmd := &Command{
		Action: strings.ToLower(fields[1]),
	}

	if len(fields) >= 3 {
		cmd.Service = fields[2]
	}

	return cmd
}

// CommandHandler processes parsed bot commands.
type CommandHandler interface {
	HandleStatus() string
	HandleDeploy(service string) string
	HandleRestart(service string) string
	HandleConfirm(service string) string
}

type Bot struct {
	client    *model.Client4
	wsClient  *model.WebSocketClient
	token     string
	url       string
	channelID string
	channel   string
	botUserID string
	handler   CommandHandler
}

type Config struct {
	URL     string
	Token   string
	Channel string
	Handler CommandHandler
}

func New(cfg Config) *Bot {
	return &Bot{
		url:     cfg.URL,
		token:   cfg.Token,
		channel: cfg.Channel,
		handler: cfg.Handler,
	}
}

func (b *Bot) Run(ctx context.Context) error {
	b.client = model.NewAPIv4Client(b.url)
	b.client.SetToken(b.token)

	// Verify connection and get bot user
	user, _, err := b.client.GetMe(ctx, "")
	if err != nil {
		return fmt.Errorf("connecting to mattermost: %w", err)
	}
	b.botUserID = user.Id

	// Find channel by name — search all teams
	teams, _, err := b.client.GetTeamsForUser(ctx, user.Id, "")
	if err != nil {
		return fmt.Errorf("getting teams: %w", err)
	}

	for _, team := range teams {
		channel, _, err := b.client.GetChannelByName(ctx, b.channel, team.Id, "")
		if err == nil {
			b.channelID = channel.Id
			break
		}
	}
	if b.channelID == "" {
		return fmt.Errorf("channel %q not found", b.channel)
	}

	// Connect websocket
	wsURL := strings.Replace(b.url, "https://", "wss://", 1)
	wsURL = strings.Replace(wsURL, "http://", "ws://", 1)

	b.wsClient, err = model.NewWebSocketClient4(wsURL, b.token)
	if err != nil {
		return fmt.Errorf("websocket connection: %w", err)
	}

	b.wsClient.Listen()

	log.Printf("Bot connected to %s, watching channel %s", b.url, b.channel)

	for {
		select {
		case <-ctx.Done():
			b.wsClient.Close()
			return nil
		case event := <-b.wsClient.EventChannel:
			if event == nil {
				continue
			}
			if event.EventType() == model.WebsocketEventPosted {
				b.handlePost(ctx, event)
			}
		}
	}
}

func (b *Bot) handlePost(ctx context.Context, event *model.WebSocketEvent) {
	post, err := model.PostFromJson(strings.NewReader(event.GetData()["post"].(string)))
	if err != nil {
		return
	}

	// Ignore own messages and messages from other channels
	if post.UserId == b.botUserID || post.ChannelId != b.channelID {
		return
	}

	cmd := ParseCommand(post.Message)
	if cmd == nil {
		return
	}

	var response string
	switch cmd.Action {
	case "status":
		response = b.handler.HandleStatus()
	case "deploy":
		if cmd.Service == "" {
			response = "Usage: `@matterops deploy <service>`"
		} else {
			response = b.handler.HandleDeploy(cmd.Service)
		}
	case "restart":
		if cmd.Service == "" {
			response = "Usage: `@matterops restart <service>`"
		} else {
			response = b.handler.HandleRestart(cmd.Service)
		}
	case "confirm":
		if cmd.Service == "" {
			response = "Usage: `@matterops confirm <service>`"
		} else {
			response = b.handler.HandleConfirm(cmd.Service)
		}
	default:
		response = fmt.Sprintf("Unknown command: `%s`. Try `status`, `deploy`, `restart`, or `confirm`.", cmd.Action)
	}

	if response != "" {
		b.PostMessage(ctx, response)
	}
}

func (b *Bot) PostMessage(ctx context.Context, message string) {
	post := &model.Post{
		ChannelId: b.channelID,
		Message:   message,
	}
	_, _, err := b.client.CreatePost(ctx, post)
	if err != nil {
		log.Printf("Error posting message: %v", err)
	}
}
```

- [ ] **Step 4: Install Mattermost client dependency**

```bash
go get github.com/mattermost/mattermost/server/public/model
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/bot/...
```

Expected: PASS (only ParseCommand is tested; the Bot itself requires a live Mattermost instance and will be tested via integration tests).

- [ ] **Step 6: Commit**

```bash
git add internal/bot/ go.mod go.sum
git commit -m "feat: add Mattermost bot with command parsing and websocket listener"
```

---

### Task 12: Web Dashboard

**Files:**
- Create: `internal/dashboard/dashboard.go`
- Create: `internal/dashboard/dashboard_test.go`
- Create: `templates/index.html`

- [ ] **Step 1: Write failing test for dashboard handler**

Create `internal/dashboard/dashboard_test.go`:

```go
package dashboard_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/dmcleish91/matterops/internal/dashboard"
	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStateProvider struct {
	states map[string]service.ServiceState
}

func (m *mockStateProvider) GetAllStates() map[string]service.ServiceState {
	return m.states
}

func TestDashboard_RendersServices(t *testing.T) {
	provider := &mockStateProvider{
		states: map[string]service.ServiceState{
			"myapp": {
				Status:     "running",
				LastDeploy: time.Date(2026, 4, 6, 12, 0, 0, 0, time.UTC),
				LastResult: "success",
				LastOutput: "$ echo deployed\ndeployed\n",
			},
			"api": {
				Status:     "failed",
				LastDeploy: time.Date(2026, 4, 6, 11, 0, 0, 0, time.UTC),
				LastResult: "failed",
				LastOutput: "$ make test\nFAIL\n",
				FailedStep: "make test",
			},
		},
	}

	d, err := dashboard.New(provider, "templates")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	d.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, "myapp")
	assert.Contains(t, body, "running")
	assert.Contains(t, body, "api")
	assert.Contains(t, body, "failed")
}

func TestDashboard_JSONEndpoint(t *testing.T) {
	provider := &mockStateProvider{
		states: map[string]service.ServiceState{
			"myapp": {Status: "running"},
		},
	}

	d, err := dashboard.New(provider, "templates")
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	rr := httptest.NewRecorder()
	d.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Header().Get("Content-Type"), "application/json")
	assert.Contains(t, rr.Body.String(), "myapp")
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/dashboard/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Create HTML template**

Create `templates/index.html`:

```html
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>MatterOps</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: #f5f5f5; color: #333; padding: 2rem; }
        h1 { margin-bottom: 1.5rem; }
        table { width: 100%; border-collapse: collapse; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1); }
        th, td { padding: 0.75rem 1rem; text-align: left; border-bottom: 1px solid #eee; }
        th { background: #fafafa; font-weight: 600; }
        .status { display: inline-block; padding: 0.25rem 0.75rem; border-radius: 4px; font-size: 0.85rem; font-weight: 500; }
        .status-running { background: #d4edda; color: #155724; }
        .status-failed { background: #f8d7da; color: #721c24; }
        .status-deploying { background: #fff3cd; color: #856404; }
        .status-stopped { background: #e2e3e5; color: #383d41; }
        .output { display: none; padding: 1rem; background: #1e1e1e; color: #d4d4d4; font-family: monospace; font-size: 0.85rem; white-space: pre-wrap; margin: 0; }
        .output.visible { display: block; }
        .toggle { cursor: pointer; user-select: none; }
        .toggle:hover { background: #f0f0f0; }
        .failed-step { color: #dc3545; font-size: 0.85rem; }
        .timestamp { color: #888; font-size: 0.85rem; }
        .refresh-note { color: #888; font-size: 0.85rem; margin-bottom: 1rem; }
    </style>
</head>
<body>
    <h1>MatterOps</h1>
    <p class="refresh-note">Auto-refreshes every 10 seconds</p>
    <table>
        <thead>
            <tr>
                <th>Service</th>
                <th>Status</th>
                <th>Last Deploy</th>
                <th>Result</th>
                <th>Details</th>
            </tr>
        </thead>
        <tbody>
            {{range $name, $state := .Services}}
            <tr class="toggle" onclick="toggleOutput('output-{{$name}}')">
                <td><strong>{{$name}}</strong></td>
                <td><span class="status status-{{$state.Status}}">{{$state.Status}}</span></td>
                <td class="timestamp">{{if $state.LastDeploy.IsZero}}&mdash;{{else}}{{$state.LastDeploy.Format "2006-01-02 15:04:05"}}{{end}}</td>
                <td>{{if $state.LastResult}}{{$state.LastResult}}{{else}}&mdash;{{end}}</td>
                <td>{{if $state.FailedStep}}<span class="failed-step">Failed: {{$state.FailedStep}}</span>{{else}}&mdash;{{end}}</td>
            </tr>
            <tr>
                <td colspan="5">
                    <pre class="output" id="output-{{$name}}">{{$state.LastOutput}}</pre>
                </td>
            </tr>
            {{end}}
        </tbody>
    </table>
    <script>
        function toggleOutput(id) {
            document.getElementById(id).classList.toggle('visible');
        }
        setTimeout(function() { location.reload(); }, 10000);
    </script>
</body>
</html>
```

- [ ] **Step 4: Implement dashboard handler**

Create `internal/dashboard/dashboard.go`:

```go
package dashboard

import (
	"encoding/json"
	"html/template"
	"net/http"
	"path/filepath"

	"github.com/dmcleish91/matterops/internal/service"
)

type StateProvider interface {
	GetAllStates() map[string]service.ServiceState
}

type Dashboard struct {
	mux      *http.ServeMux
	provider StateProvider
	tmpl     *template.Template
}

func New(provider StateProvider, templatesDir string) (*Dashboard, error) {
	tmpl, err := template.ParseFiles(filepath.Join(templatesDir, "index.html"))
	if err != nil {
		return nil, err
	}

	d := &Dashboard{
		mux:      http.NewServeMux(),
		provider: provider,
		tmpl:     tmpl,
	}

	d.mux.HandleFunc("/", d.handleIndex)
	d.mux.HandleFunc("/api/status", d.handleAPIStatus)

	return d, nil
}

func (d *Dashboard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mux.ServeHTTP(w, r)
}

func (d *Dashboard) handleIndex(w http.ResponseWriter, r *http.Request) {
	data := struct {
		Services map[string]service.ServiceState
	}{
		Services: d.provider.GetAllStates(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := d.tmpl.Execute(w, data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (d *Dashboard) handleAPIStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(d.provider.GetAllStates())
}
```

- [ ] **Step 5: Run tests**

Note: tests need the templates dir relative to the test file. Update the test to use the project root templates:

```bash
go test ./internal/dashboard/... -run TestDashboard
```

The test passes `"templates"` but runs from `internal/dashboard/`. We need to pass the absolute path. Update the test to use a helper:

Actually, the simplest fix: in the test, create a temporary template. Replace the `New` calls in the test with the actual templates path:

```go
d, err := dashboard.New(provider, "../../templates")
```

Run:

```bash
go test ./internal/dashboard/...
```

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/dashboard/ templates/
git commit -m "feat: add web dashboard with status page and JSON API"
```

---

### Task 13: Systemctl Backend

**Files:**
- Create: `internal/service/systemctl.go`
- Create: `internal/service/systemctl_test.go`

- [ ] **Step 1: Write failing test for systemctl backend**

Create `internal/service/systemctl_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestSystemctlBackend_CommandFormation(t *testing.T) {
	// We can't test actual systemctl calls in CI, but we can test
	// that the backend implements the interface and forms correct commands.
	b := service.NewSystemctlBackend("myapp")
	assert.Implements(t, (*service.Backend)(nil), b)

	// Verify it returns an error on non-Linux (or when systemctl isn't available)
	_, err := b.Status(context.Background())
	// On macOS in dev, this will fail — that's expected
	if err != nil {
		assert.Contains(t, err.Error(), "systemctl")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/... -run TestSystemctl
```

Expected: FAIL — `NewSystemctlBackend` not defined.

- [ ] **Step 3: Implement systemctl backend**

Create `internal/service/systemctl.go`:

```go
package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type SystemctlBackend struct {
	unit string
}

func NewSystemctlBackend(unit string) *SystemctlBackend {
	return &SystemctlBackend{unit: unit}
}

func (s *SystemctlBackend) Start(ctx context.Context) error {
	return s.run(ctx, "start")
}

func (s *SystemctlBackend) Stop(ctx context.Context) error {
	return s.run(ctx, "stop")
}

func (s *SystemctlBackend) Restart(ctx context.Context) error {
	return s.run(ctx, "restart")
}

func (s *SystemctlBackend) Status(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", s.unit)
	out, err := cmd.Output()
	status := strings.TrimSpace(string(out))

	if status == "active" {
		return "running", nil
	}
	if status == "inactive" || status == "dead" {
		return "stopped", nil
	}
	if status == "failed" {
		return "failed", nil
	}

	if err != nil {
		return "stopped", fmt.Errorf("systemctl is-active %s: %w", s.unit, err)
	}
	return "stopped", nil
}

func (s *SystemctlBackend) run(ctx context.Context, action string) error {
	cmd := exec.CommandContext(ctx, "systemctl", action, s.unit)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %s %s: %s: %w", action, s.unit, string(out), err)
	}
	return nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/service/... -run TestSystemctl
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/systemctl.go internal/service/systemctl_test.go
git commit -m "feat: add systemctl backend for Linux service management"
```

---

### Task 14: Launchctl Backend

**Files:**
- Create: `internal/service/launchctl.go`
- Create: `internal/service/launchctl_test.go`

- [ ] **Step 1: Write failing test for launchctl backend**

Create `internal/service/launchctl_test.go`:

```go
package service_test

import (
	"context"
	"testing"

	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestLaunchctlBackend_Interface(t *testing.T) {
	b := service.NewLaunchctlBackend("com.example.myapp")
	assert.Implements(t, (*service.Backend)(nil), b)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/... -run TestLaunchctl
```

Expected: FAIL — `NewLaunchctlBackend` not defined.

- [ ] **Step 3: Implement launchctl backend**

Create `internal/service/launchctl.go`:

```go
package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type LaunchctlBackend struct {
	label string
}

func NewLaunchctlBackend(label string) *LaunchctlBackend {
	return &LaunchctlBackend{label: label}
}

func (l *LaunchctlBackend) Start(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "launchctl", "start", l.label)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl start %s: %s: %w", l.label, string(out), err)
	}
	return nil
}

func (l *LaunchctlBackend) Stop(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "launchctl", "stop", l.label)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl stop %s: %s: %w", l.label, string(out), err)
	}
	return nil
}

func (l *LaunchctlBackend) Restart(ctx context.Context) error {
	if err := l.Stop(ctx); err != nil {
		return err
	}
	return l.Start(ctx)
}

func (l *LaunchctlBackend) Status(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "launchctl", "list")
	out, err := cmd.Output()
	if err != nil {
		return "stopped", fmt.Errorf("launchctl list: %w", err)
	}

	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, l.label) {
			fields := strings.Fields(line)
			if len(fields) >= 2 && fields[1] == "0" {
				return "running", nil
			}
			return "running", nil
		}
	}

	return "stopped", nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/service/... -run TestLaunchctl
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/launchctl.go internal/service/launchctl_test.go
git commit -m "feat: add launchctl backend for macOS service management"
```

---

### Task 15: Wire Backend Selection into Manager

**Files:**
- Modify: `internal/service/manager.go`
- Modify: `internal/service/manager_test.go`

- [ ] **Step 1: Write failing test for backend selection**

Add to `internal/service/manager_test.go`:

```go
func TestManager_BackendSelection(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		config   config.ServiceConfig
		wantType string
	}{
		{
			name: "process backend",
			config: config.ServiceConfig{
				Name:       "proc-svc",
				WorkingDir: dir,
				Process:    config.ProcessConfig{Cmd: "sleep 60"},
			},
			wantType: "*service.ProcessBackend",
		},
		{
			name: "service_name on linux defaults to systemctl",
			config: config.ServiceConfig{
				Name:        "sys-svc",
				WorkingDir:  dir,
				ServiceName: "myunit",
			},
			// On macOS this will be LaunchctlBackend, on Linux SystemctlBackend
			// Just verify it doesn't panic and creates a manager
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := service.NewManager([]config.ServiceConfig{tt.config}, nil)
			require.NoError(t, err)
			defer m.Stop()

			_, ok := m.GetState(tt.config.Name)
			assert.True(t, ok)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it passes (or update backendForConfig)**

Update `backendForConfig` in `internal/service/manager.go`:

```go
import "runtime"

func backendForConfig(cfg config.ServiceConfig) Backend {
	if cfg.Process.Cmd != "" {
		return NewProcessBackend(cfg.Process.Cmd, cfg.WorkingDir)
	}
	if cfg.ServiceName != "" {
		if runtime.GOOS == "darwin" {
			return NewLaunchctlBackend(cfg.ServiceName)
		}
		return NewSystemctlBackend(cfg.ServiceName)
	}
	// Fallback: process with no-op
	return NewProcessBackend("echo no backend configured", cfg.WorkingDir)
}
```

- [ ] **Step 3: Run tests**

```bash
go test -race ./internal/service/...
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add internal/service/manager.go internal/service/manager_test.go
git commit -m "feat: wire backend selection into manager (process, systemctl, launchctl)"
```

---

### Task 16: Confirmation Flow

**Files:**
- Create: `internal/service/confirmation.go`
- Create: `internal/service/confirmation_test.go`

- [ ] **Step 1: Write failing test for confirmation tracking**

Create `internal/service/confirmation_test.go`:

```go
package service_test

import (
	"testing"
	"time"

	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestConfirmationTracker_PendAndConfirm(t *testing.T) {
	ct := service.NewConfirmationTracker(10 * time.Minute)

	ct.AddPending("myapp", "abc123")
	assert.True(t, ct.IsPending("myapp"))

	ok := ct.Confirm("myapp")
	assert.True(t, ok)
	assert.False(t, ct.IsPending("myapp"))
}

func TestConfirmationTracker_ConfirmNonPending(t *testing.T) {
	ct := service.NewConfirmationTracker(10 * time.Minute)

	ok := ct.Confirm("myapp")
	assert.False(t, ok)
}

func TestConfirmationTracker_Expiry(t *testing.T) {
	ct := service.NewConfirmationTracker(1 * time.Millisecond)

	ct.AddPending("myapp", "abc123")
	time.Sleep(10 * time.Millisecond)

	assert.False(t, ct.IsPending("myapp"))
	ok := ct.Confirm("myapp")
	assert.False(t, ok)
}

func TestConfirmationTracker_OverwritesPending(t *testing.T) {
	ct := service.NewConfirmationTracker(10 * time.Minute)

	ct.AddPending("myapp", "commit1")
	ct.AddPending("myapp", "commit2")

	assert.True(t, ct.IsPending("myapp"))
	ok := ct.Confirm("myapp")
	assert.True(t, ok)
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/service/... -run TestConfirmation
```

Expected: FAIL — `NewConfirmationTracker` not defined.

- [ ] **Step 3: Implement confirmation tracker**

Create `internal/service/confirmation.go`:

```go
package service

import (
	"sync"
	"time"
)

type pendingConfirmation struct {
	commit    string
	createdAt time.Time
}

type ConfirmationTracker struct {
	mu      sync.Mutex
	pending map[string]pendingConfirmation
	timeout time.Duration
}

func NewConfirmationTracker(timeout time.Duration) *ConfirmationTracker {
	return &ConfirmationTracker{
		pending: make(map[string]pendingConfirmation),
		timeout: timeout,
	}
}

func (ct *ConfirmationTracker) AddPending(serviceName string, commit string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	ct.pending[serviceName] = pendingConfirmation{
		commit:    commit,
		createdAt: time.Now(),
	}
}

func (ct *ConfirmationTracker) IsPending(serviceName string) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	p, ok := ct.pending[serviceName]
	if !ok {
		return false
	}

	if time.Since(p.createdAt) > ct.timeout {
		delete(ct.pending, serviceName)
		return false
	}

	return true
}

func (ct *ConfirmationTracker) Confirm(serviceName string) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()

	p, ok := ct.pending[serviceName]
	if !ok {
		return false
	}

	if time.Since(p.createdAt) > ct.timeout {
		delete(ct.pending, serviceName)
		return false
	}

	delete(ct.pending, serviceName)
	return true
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./internal/service/... -run TestConfirmation
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/service/confirmation.go internal/service/confirmation_test.go
git commit -m "feat: add confirmation tracker with expiry for gated deployments"
```

---

### Task 17: Main Entrypoint — Wire Everything Together

**Files:**
- Modify: `cmd/matterops/main.go`
- Create: `internal/app/app.go`
- Create: `internal/app/app_test.go`

- [ ] **Step 1: Write failing test for app initialization**

Create `internal/app/app_test.go`:

```go
package app_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/dmcleish91/matterops/internal/app"
	"github.com/stretchr/testify/require"
)

func TestNewApp_LoadsConfig(t *testing.T) {
	dir := t.TempDir()

	// Write config
	cfgPath := filepath.Join(dir, "config.yaml")
	svcDir := filepath.Join(dir, "services")
	require.NoError(t, os.MkdirAll(svcDir, 0755))

	require.NoError(t, os.WriteFile(cfgPath, []byte(`
mattermost:
  url: "http://localhost:8065"
  channel: "ops"
webhook:
  port: 0
dashboard:
  port: 0
services_dir: "`+svcDir+`"
`), 0644))

	require.NoError(t, os.WriteFile(filepath.Join(svcDir, "test.yaml"), []byte(`
branch: main
repo: "github.com/org/test"
working_dir: "`+dir+`"
deploy:
  - "echo hello"
process:
  cmd: "sleep 60"
`), 0644))

	// Write .env
	envPath := filepath.Join(dir, ".env")
	require.NoError(t, os.WriteFile(envPath, []byte(`
MATTERMOST_TOKEN=test-token
GITHUB_WEBHOOK_SECRET=test-secret
`), 0644))

	a, err := app.New(cfgPath, envPath)
	require.NoError(t, err)
	require.NotNil(t, a)
	defer a.Shutdown()
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./internal/app/...
```

Expected: FAIL — package doesn't exist.

- [ ] **Step 3: Implement app package**

Create `internal/app/app.go`:

```go
package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/dmcleish91/matterops/internal/bot"
	"github.com/dmcleish91/matterops/internal/config"
	"github.com/dmcleish91/matterops/internal/dashboard"
	"github.com/dmcleish91/matterops/internal/service"
	"github.com/dmcleish91/matterops/internal/webhook"
)

type App struct {
	cfg           *config.Config
	env           *config.Env
	manager       *service.Manager
	confirmations *service.ConfirmationTracker
	bot           *bot.Bot
	webhookSrv    *http.Server
	dashboardSrv  *http.Server
	cancel        context.CancelFunc
}

func New(configPath string, envPath string) (*App, error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	env, err := config.LoadEnv(envPath)
	if err != nil {
		return nil, fmt.Errorf("loading env: %w", err)
	}

	services, err := config.LoadServices(cfg.ServicesDir)
	if err != nil {
		return nil, fmt.Errorf("loading services: %w", err)
	}

	confirmations := service.NewConfirmationTracker(10 * time.Minute)

	mgr, err := service.NewManager(services, nil)
	if err != nil {
		return nil, fmt.Errorf("creating manager: %w", err)
	}

	a := &App{
		cfg:           cfg,
		env:           env,
		manager:       mgr,
		confirmations: confirmations,
	}

	return a, nil
}

func (a *App) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	a.cancel = cancel

	// Start webhook server
	whHandler := webhook.NewHandler(a.env.WebhookSecret, a)
	a.webhookSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.cfg.Webhook.Port),
		Handler: whHandler,
	}
	go func() {
		log.Printf("Webhook server listening on :%d", a.cfg.Webhook.Port)
		if err := a.webhookSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Webhook server error: %v", err)
		}
	}()

	// Start dashboard server
	dash, err := dashboard.New(a.manager, "templates")
	if err != nil {
		return fmt.Errorf("creating dashboard: %w", err)
	}
	a.dashboardSrv = &http.Server{
		Addr:    fmt.Sprintf(":%d", a.cfg.Dashboard.Port),
		Handler: dash,
	}
	go func() {
		log.Printf("Dashboard listening on :%d", a.cfg.Dashboard.Port)
		if err := a.dashboardSrv.ListenAndServe(); err != http.ErrServerClosed {
			log.Printf("Dashboard server error: %v", err)
		}
	}()

	// Start Mattermost bot
	a.bot = bot.New(bot.Config{
		URL:     a.cfg.Mattermost.URL,
		Token:   a.env.MattermostToken,
		Channel: a.cfg.Mattermost.Channel,
		Handler: a,
	})
	go func() {
		if err := a.bot.Run(ctx); err != nil {
			log.Printf("Bot error: %v", err)
		}
	}()

	<-ctx.Done()
	return nil
}

func (a *App) Shutdown() {
	if a.cancel != nil {
		a.cancel()
	}
	if a.webhookSrv != nil {
		a.webhookSrv.Close()
	}
	if a.dashboardSrv != nil {
		a.dashboardSrv.Close()
	}
	if a.manager != nil {
		a.manager.Stop()
	}
}

// HandlePush implements webhook.DeployTrigger
func (a *App) HandlePush(repo string, branch string) {
	name, ok := a.manager.FindServiceByRepo(repo, branch)
	if !ok {
		log.Printf("No service configured for %s/%s", repo, branch)
		return
	}

	cfg, _ := a.manager.GetServiceConfig(name)
	if cfg.RequireConfirmation {
		a.confirmations.AddPending(name, "")
		if a.bot != nil {
			msg := fmt.Sprintf("Push to `%s` on `%s`. Deploy? Reply `@matterops confirm %s`", branch, name, name)
			a.bot.PostMessage(context.Background(), msg)
		}
		return
	}

	if err := a.manager.RequestDeploy(name); err != nil {
		log.Printf("Error requesting deploy for %s: %v", name, err)
	}
}

// HandleStatus implements bot.CommandHandler
func (a *App) HandleStatus() string {
	states := a.manager.GetAllStates()
	if len(states) == 0 {
		return "No services configured."
	}

	var lines []string
	for name, state := range states {
		line := fmt.Sprintf("**%s**: %s", name, state.Status)
		if state.LastResult != "" {
			line += fmt.Sprintf(" (last deploy: %s)", state.LastResult)
		}
		lines = append(lines, line)
	}

	result := "**Service Status:**\n"
	for _, l := range lines {
		result += "- " + l + "\n"
	}
	return result
}

// HandleDeploy implements bot.CommandHandler
func (a *App) HandleDeploy(svc string) string {
	if err := a.manager.RequestDeploy(svc); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Deploy requested for `%s`.", svc)
}

// HandleRestart implements bot.CommandHandler
func (a *App) HandleRestart(svc string) string {
	if err := a.manager.RestartService(svc); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("`%s` restarted.", svc)
}

// HandleConfirm implements bot.CommandHandler
func (a *App) HandleConfirm(svc string) string {
	if !a.confirmations.Confirm(svc) {
		return fmt.Sprintf("No pending deployment for `%s` (may have expired).", svc)
	}
	if err := a.manager.RequestDeploy(svc); err != nil {
		return fmt.Sprintf("Error: %v", err)
	}
	return fmt.Sprintf("Confirmed. Deploying `%s`.", svc)
}
```

- [ ] **Step 4: Update main.go**

Replace `cmd/matterops/main.go`:

```go
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/dmcleish91/matterops/internal/app"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	envPath := flag.String("env", ".env", "path to .env file")
	flag.Parse()

	a, err := app.New(*configPath, *envPath)
	if err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		a.Shutdown()
		cancel()
	}()

	if err := a.Run(ctx); err != nil {
		log.Fatalf("Error: %v", err)
	}
}
```

- [ ] **Step 5: Run tests**

```bash
go test ./internal/app/...
```

Expected: PASS

- [ ] **Step 6: Run make check**

```bash
make check
```

Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add cmd/matterops/ internal/app/ go.mod go.sum
git commit -m "feat: wire all components together in app package and main entrypoint"
```

---

### Task 18: Dev Configuration and Dummy Services

**Files:**
- Create: `config.dev.yaml`
- Create: `services/echo.yaml`

- [ ] **Step 1: Create dev config**

Create `config.dev.yaml`:

```yaml
mattermost:
  url: "http://localhost:8065"
  channel: "ops"

webhook:
  port: 8080

dashboard:
  port: 8081

services_dir: "./services"
```

- [ ] **Step 2: Create dummy service config**

Create `services/echo.yaml`:

```yaml
branch: main
repo: "github.com/example/echo"
working_dir: "."
deploy:
  - "echo 'pulling latest...'"
  - "echo 'running tests...'"
  - "echo 'building...'"
process:
  cmd: "echo 'echo service running' && sleep 3600"
```

- [ ] **Step 3: Create .env for dev**

Create `.env`:

```
MATTERMOST_TOKEN=dev-token
GITHUB_WEBHOOK_SECRET=dev-secret
```

- [ ] **Step 4: Verify build works**

```bash
make build
```

Expected: PASS — binary at `bin/matterops`

- [ ] **Step 5: Commit**

```bash
git add config.dev.yaml services/echo.yaml
git commit -m "feat: add dev configuration and dummy echo service"
```

---

### Task 19: Playwright Dashboard Tests

**Files:**
- Create: `playwright.config.ts`
- Create: `package.json`
- Create: `tests/dashboard.spec.ts`

- [ ] **Step 1: Initialize Playwright**

```bash
npm init -y
npm install -D @playwright/test
npx playwright install chromium
```

- [ ] **Step 2: Create Playwright config**

Create `playwright.config.ts`:

```typescript
import { defineConfig } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  timeout: 30000,
  use: {
    baseURL: 'http://localhost:8081',
  },
});
```

- [ ] **Step 3: Write dashboard tests**

Create `tests/dashboard.spec.ts`:

```typescript
import { test, expect } from '@playwright/test';

test.describe('MatterOps Dashboard', () => {
  test('shows page title', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('h1')).toHaveText('MatterOps');
  });

  test('shows service table', async ({ page }) => {
    await page.goto('/');
    await expect(page.locator('table')).toBeVisible();
    await expect(page.locator('th').first()).toHaveText('Service');
  });

  test('shows service status', async ({ page }) => {
    await page.goto('/');
    // At minimum, the echo service from dev config should appear
    await expect(page.locator('td strong')).toContainText(['echo']);
  });

  test('has JSON API endpoint', async ({ request }) => {
    const response = await request.get('/api/status');
    expect(response.ok()).toBeTruthy();
    const data = await response.json();
    expect(data).toHaveProperty('echo');
  });

  test('clicking a service row toggles output', async ({ page }) => {
    await page.goto('/');
    const outputPre = page.locator('.output').first();
    await expect(outputPre).not.toBeVisible();

    await page.locator('.toggle').first().click();
    await expect(outputPre).toBeVisible();

    await page.locator('.toggle').first().click();
    await expect(outputPre).not.toBeVisible();
  });
});
```

- [ ] **Step 4: Add node_modules to .gitignore**

Append to `.gitignore`:

```
node_modules/
```

- [ ] **Step 5: Verify Playwright config works**

```bash
npx playwright test --list
```

Expected: Lists the 5 test cases.

- [ ] **Step 6: Commit**

```bash
git add playwright.config.ts package.json package-lock.json tests/ .gitignore
git commit -m "feat: add Playwright tests for web dashboard"
```

---

### Task 20: Manager Notifier Integration

**Files:**
- Create: `internal/service/notifier_test.go`
- Modify: `internal/service/manager_test.go`

- [ ] **Step 1: Write failing test for notifier callbacks during deploy**

Create `internal/service/notifier_test.go`:

```go
package service_test

import (
	"testing"
	"time"

	"github.com/dmcleish91/matterops/internal/config"
	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingNotifier struct {
	started   []string
	succeeded []string
	failed    []string
	queued    []string
}

func (n *recordingNotifier) DeployStarted(svc string) error {
	n.started = append(n.started, svc)
	return nil
}
func (n *recordingNotifier) DeploySucceeded(svc string, output string) error {
	n.succeeded = append(n.succeeded, svc)
	return nil
}
func (n *recordingNotifier) DeployFailed(svc string, step string, output string) error {
	n.failed = append(n.failed, svc)
	return nil
}
func (n *recordingNotifier) DeployQueued(svc string) error {
	n.queued = append(n.queued, svc)
	return nil
}

func TestManager_NotifiesOnDeploy(t *testing.T) {
	dir := t.TempDir()
	notifier := &recordingNotifier{}
	services := []config.ServiceConfig{
		{
			Name:       "notifyapp",
			Branch:     "main",
			WorkingDir: dir,
			Deploy:     []string{"echo ok"},
			Process:    config.ProcessConfig{Cmd: "sleep 60"},
		},
	}

	m, err := service.NewManager(services, notifier)
	require.NoError(t, err)
	defer m.Stop()

	err = m.RequestDeploy("notifyapp")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(notifier.succeeded) > 0
	}, 5*time.Second, 50*time.Millisecond)

	assert.Contains(t, notifier.queued, "notifyapp")
	assert.Contains(t, notifier.started, "notifyapp")
	assert.Contains(t, notifier.succeeded, "notifyapp")
}

func TestManager_NotifiesOnFailedDeploy(t *testing.T) {
	dir := t.TempDir()
	notifier := &recordingNotifier{}
	services := []config.ServiceConfig{
		{
			Name:       "failapp",
			Branch:     "main",
			WorkingDir: dir,
			Deploy:     []string{"exit 1"},
			Process:    config.ProcessConfig{Cmd: "sleep 60"},
		},
	}

	m, err := service.NewManager(services, notifier)
	require.NoError(t, err)
	defer m.Stop()

	err = m.RequestDeploy("failapp")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(notifier.failed) > 0
	}, 5*time.Second, 50*time.Millisecond)

	assert.Contains(t, notifier.started, "failapp")
	assert.Contains(t, notifier.failed, "failapp")
	assert.Empty(t, notifier.succeeded)
}
```

- [ ] **Step 2: Run tests**

```bash
go test -race ./internal/service/... -run TestManager_Notif
```

Expected: PASS (the notifier is already wired in the manager from Task 8).

- [ ] **Step 3: Commit**

```bash
git add internal/service/notifier_test.go
git commit -m "test: add notifier integration tests for deploy lifecycle"
```

---

### Task 21: Final Integration — Make Check Passes

**Files:**
- Potentially fix any remaining compilation or test issues

- [ ] **Step 1: Run full check**

```bash
make check
```

- [ ] **Step 2: Fix any issues found**

Address any lint warnings, compilation errors, or test failures.

- [ ] **Step 3: Verify all tests pass with race detection**

```bash
go test -race -count=1 ./...
```

Expected: All tests PASS.

- [ ] **Step 4: Verify binary runs and exits cleanly**

```bash
timeout 2 ./bin/matterops --config config.dev.yaml --env .env 2>&1 || true
```

Expected: Starts up, prints log messages, exits on timeout.

- [ ] **Step 5: Commit any fixes**

```bash
git add -A
git commit -m "fix: resolve integration issues and ensure make check passes"
```
