package service_test

import (
	"sync"
	"testing"
	"time"

	"github.com/dmcleish91/matterops/internal/config"
	"github.com/dmcleish91/matterops/internal/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingNotifier struct {
	mu        sync.Mutex
	started   []string
	succeeded []string
	failed    []string
	queued    []string
}

func (n *recordingNotifier) DeployStarted(svc string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.started = append(n.started, svc)
	return nil
}
func (n *recordingNotifier) DeploySucceeded(svc string, output string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.succeeded = append(n.succeeded, svc)
	return nil
}
func (n *recordingNotifier) DeployFailed(svc string, step string, output string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.failed = append(n.failed, svc)
	return nil
}
func (n *recordingNotifier) DeployQueued(svc string) error {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.queued = append(n.queued, svc)
	return nil
}

func (n *recordingNotifier) getSucceeded() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]string, len(n.succeeded))
	copy(result, n.succeeded)
	return result
}

func (n *recordingNotifier) getFailed() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]string, len(n.failed))
	copy(result, n.failed)
	return result
}

func (n *recordingNotifier) getStarted() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]string, len(n.started))
	copy(result, n.started)
	return result
}

func (n *recordingNotifier) getQueued() []string {
	n.mu.Lock()
	defer n.mu.Unlock()
	result := make([]string, len(n.queued))
	copy(result, n.queued)
	return result
}

func TestManager_NotifiesOnDeploy(t *testing.T) {
	dir := t.TempDir()
	notifier := &recordingNotifier{}
	services := []config.ServiceConfig{
		{
			Name: "notifyapp", Branch: "main", WorkingDir: dir,
			Deploy:  []string{"echo ok"},
			Process: config.ProcessConfig{Cmd: "sleep 60"},
		},
	}
	m, err := service.NewManager(services, notifier)
	require.NoError(t, err)
	defer m.Stop()

	err = m.RequestDeploy("notifyapp")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(notifier.getSucceeded()) > 0
	}, 5*time.Second, 50*time.Millisecond)

	assert.Contains(t, notifier.getQueued(), "notifyapp")
	assert.Contains(t, notifier.getStarted(), "notifyapp")
	assert.Contains(t, notifier.getSucceeded(), "notifyapp")
}

func TestManager_NotifiesOnFailedDeploy(t *testing.T) {
	dir := t.TempDir()
	notifier := &recordingNotifier{}
	services := []config.ServiceConfig{
		{
			Name: "failapp", Branch: "main", WorkingDir: dir,
			Deploy:  []string{"exit 1"},
			Process: config.ProcessConfig{Cmd: "sleep 60"},
		},
	}
	m, err := service.NewManager(services, notifier)
	require.NoError(t, err)
	defer m.Stop()

	err = m.RequestDeploy("failapp")
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		return len(notifier.getFailed()) > 0
	}, 5*time.Second, 50*time.Millisecond)

	assert.Contains(t, notifier.getStarted(), "failapp")
	assert.Contains(t, notifier.getFailed(), "failapp")
	assert.Empty(t, notifier.getSucceeded())
}
