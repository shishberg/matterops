package service

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/shishberg/matterops/internal/config"
	"github.com/shishberg/matterops/internal/deploy"
)

// Notifier receives deploy lifecycle events.
type Notifier interface {
	DeployStarted(serviceName string) error
	DeploySucceeded(serviceName string, output string) error
	DeployFailed(serviceName string, failedStep string, output string) error
	DeployQueued(serviceName string) error
}

// ServiceState holds the current state of a managed service.
type ServiceState struct {
	Status     string    `json:"status"`
	LastDeploy time.Time `json:"last_deploy"`
	LastResult string    `json:"last_result"`
	LastOutput string    `json:"last_output"`
	FailedStep string    `json:"failed_step"`
}

type managedService struct {
	config   config.ServiceConfig
	backend  Backend
	state    ServiceState
	mu       sync.Mutex
	deployCh chan struct{}
}

// Manager manages a set of services.
type Manager struct {
	services map[string]*managedService
	notifier Notifier
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// backendForConfig selects the appropriate backend based on the service config.
func backendForConfig(cfg config.ServiceConfig) Backend {
	if cfg.Process.Cmd != "" {
		return NewProcessBackend(cfg.Process.Cmd, cfg.WorkingDir)
	}
	if cfg.ServiceName != "" {
		if runtime.GOOS == "darwin" {
			return NewLaunchctlBackend(cfg.ServiceName)
		}
		return NewSystemctlBackend(cfg.ServiceName, cfg.UserService)
	}
	// Fallback
	return NewProcessBackend("echo no backend configured", cfg.WorkingDir)
}

// NewManager creates a Manager for the given service configs.
func NewManager(services []config.ServiceConfig, notifier Notifier) (*Manager, error) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		services: make(map[string]*managedService, len(services)),
		notifier: notifier,
		ctx:      ctx,
		cancel:   cancel,
	}

	for _, svc := range services {
		backend := backendForConfig(svc)

		ms := &managedService{
			config:   svc,
			backend:  backend,
			state:    ServiceState{Status: "stopped"},
			deployCh: make(chan struct{}, 1),
		}
		m.services[svc.Name] = ms

		m.wg.Add(1)
		go m.runWorker(ms)
	}

	return m, nil
}

// SetNotifier sets the notifier after construction. This allows resolving the
// chicken-and-egg dependency between Manager and the bot-backed Notifier.
// Must be called before any deploys are requested (i.e. before Run).
func (m *Manager) SetNotifier(n Notifier) {
	m.notifier = n
}

// Stop shuts down all workers and backends.
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

// GetAllStates returns a snapshot of all service states.
func (m *Manager) GetAllStates() map[string]ServiceState {
	result := make(map[string]ServiceState, len(m.services))
	for name, ms := range m.services {
		ms.mu.Lock()
		result[name] = ms.state
		ms.mu.Unlock()
	}
	return result
}

// GetState returns the state for a named service.
func (m *Manager) GetState(name string) (ServiceState, bool) {
	ms, ok := m.services[name]
	if !ok {
		return ServiceState{}, false
	}
	ms.mu.Lock()
	defer ms.mu.Unlock()
	return ms.state, true
}

// GetServiceConfig returns the config for a named service.
func (m *Manager) GetServiceConfig(name string) (config.ServiceConfig, bool) {
	ms, ok := m.services[name]
	if !ok {
		return config.ServiceConfig{}, false
	}
	return ms.config, true
}

// FindServiceByRepo returns a service config matching the given repo and branch.
// repo may be "org/name" (as sent by GitHub webhooks) or "github.com/org/name"
// (as stored in service configs). Both forms are matched by normalizing the
// github.com/ prefix away before comparing.
func (m *Manager) FindServiceByRepo(repo, branch string) (config.ServiceConfig, bool) {
	normalizedRepo := strings.TrimPrefix(repo, "github.com/")
	for _, ms := range m.services {
		cfgRepo := strings.TrimPrefix(ms.config.Repo, "github.com/")
		if cfgRepo == normalizedRepo && ms.config.Branch == branch {
			return ms.config, true
		}
	}
	return config.ServiceConfig{}, false
}

// RequestDeploy queues a deploy request for the named service (latest-wins).
func (m *Manager) RequestDeploy(name string) error {
	ms, ok := m.services[name]
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	// Drain any pending deploy request (latest-wins).
	select {
	case <-ms.deployCh:
	default:
	}

	// Mark as deploying synchronously so callers see a non-idle state immediately.
	ms.mu.Lock()
	ms.state.Status = "deploying"
	ms.mu.Unlock()

	// Non-blocking send: if the channel already holds a pending request (because
	// the worker is busy and another caller raced us), that's fine — the worker
	// will pick it up.
	select {
	case ms.deployCh <- struct{}{}:
	default:
	}

	if m.notifier != nil {
		_ = m.notifier.DeployQueued(name)
	}

	return nil
}

// RestartService restarts the named service's backend and updates state to "running".
func (m *Manager) RestartService(name string) error {
	ms, ok := m.services[name]
	if !ok {
		return fmt.Errorf("service %q not found", name)
	}

	if err := ms.backend.Restart(m.ctx); err != nil {
		return fmt.Errorf("restarting service %q: %w", name, err)
	}

	ms.mu.Lock()
	ms.state.Status = "running"
	ms.mu.Unlock()

	return nil
}

// runWorker drains the deploy queue for a service until the manager is stopped.
func (m *Manager) runWorker(ms *managedService) {
	defer m.wg.Done()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ms.deployCh:
			m.executeDeploy(ms)
		}
	}
}

// executeDeploy runs the deploy steps, updates state, notifies, and restarts the service.
func (m *Manager) executeDeploy(ms *managedService) {
	name := ms.config.Name

	ms.mu.Lock()
	ms.state.Status = "deploying"
	ms.state.LastDeploy = time.Now()
	ms.mu.Unlock()

	if m.notifier != nil {
		_ = m.notifier.DeployStarted(name)
	}

	result, err := deploy.RunSteps(m.ctx, ms.config.Deploy, ms.config.WorkingDir)
	if err != nil || result.Status != "success" {
		failedStep := ""
		output := ""
		if result != nil {
			failedStep = result.FailedStep
			output = result.Output
		}

		ms.mu.Lock()
		ms.state.Status = "failed"
		ms.state.LastResult = "failed"
		ms.state.LastOutput = output
		ms.state.FailedStep = failedStep
		ms.mu.Unlock()

		if m.notifier != nil {
			_ = m.notifier.DeployFailed(name, failedStep, output)
		}
		return
	}

	if restartErr := ms.backend.Restart(m.ctx); restartErr != nil {
		ms.mu.Lock()
		ms.state.Status = "failed"
		ms.state.LastResult = "failed"
		ms.state.LastOutput = result.Output
		ms.state.FailedStep = "restart"
		ms.mu.Unlock()

		if m.notifier != nil {
			_ = m.notifier.DeployFailed(name, "restart", result.Output)
		}
		return
	}

	ms.mu.Lock()
	ms.state.Status = "running"
	ms.state.LastResult = "success"
	ms.state.LastOutput = result.Output
	ms.state.FailedStep = ""
	ms.mu.Unlock()

	if m.notifier != nil {
		_ = m.notifier.DeploySucceeded(name, result.Output)
	}
}
