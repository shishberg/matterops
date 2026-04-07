package service

import (
	"context"
	"testing"

	"github.com/shishberg/matterops/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockBackend implements Backend with a configurable Status response.
type mockBackend struct {
	status string
}

func (m *mockBackend) Start(_ context.Context) error   { return nil }
func (m *mockBackend) Stop(_ context.Context) error    { return nil }
func (m *mockBackend) Restart(_ context.Context) error { return nil }
func (m *mockBackend) Status(_ context.Context) (string, error) {
	return m.status, nil
}

func TestGetState_ProbesBackendStatus(t *testing.T) {
	// Override the backend factory so we control Status().
	origFactory := newBackend
	t.Cleanup(func() { newBackend = origFactory })

	mb := &mockBackend{status: "running"}
	newBackend = func(_ config.ServiceConfig) Backend {
		return mb
	}

	services := []config.ServiceConfig{
		{
			Name:       "myapp",
			WorkingDir: t.TempDir(),
		},
	}

	m, err := NewManager(services, nil)
	require.NoError(t, err)
	defer m.Stop()

	// Should reflect the live backend status, not a hardcoded value.
	state, ok := m.GetState("myapp")
	require.True(t, ok)
	assert.Equal(t, "running", state.Status)

	// Simulate the service crashing.
	mb.status = "stopped"
	state, ok = m.GetState("myapp")
	require.True(t, ok)
	assert.Equal(t, "stopped", state.Status)
}

func TestGetAllStates_ProbesBackendStatus(t *testing.T) {
	origFactory := newBackend
	t.Cleanup(func() { newBackend = origFactory })

	mb := &mockBackend{status: "running"}
	newBackend = func(_ config.ServiceConfig) Backend {
		return mb
	}

	services := []config.ServiceConfig{
		{
			Name:       "myapp",
			WorkingDir: t.TempDir(),
		},
	}

	m, err := NewManager(services, nil)
	require.NoError(t, err)
	defer m.Stop()

	states := m.GetAllStates()
	assert.Equal(t, "running", states["myapp"].Status)

	mb.status = "failed"
	states = m.GetAllStates()
	assert.Equal(t, "failed", states["myapp"].Status)
}

func TestGetState_PreservesDeployingStatus(t *testing.T) {
	// While a deploy is in progress, Status should show "deploying"
	// rather than probing the backend (which might say "stopped" mid-restart).
	origFactory := newBackend
	t.Cleanup(func() { newBackend = origFactory })

	mb := &mockBackend{status: "stopped"}
	newBackend = func(_ config.ServiceConfig) Backend {
		return mb
	}

	services := []config.ServiceConfig{
		{
			Name:       "myapp",
			WorkingDir: t.TempDir(),
		},
	}

	m, err := NewManager(services, nil)
	require.NoError(t, err)
	defer m.Stop()

	// Simulate deploying state (set by RequestDeploy).
	ms := m.services["myapp"]
	ms.mu.Lock()
	ms.state.Status = "deploying"
	ms.mu.Unlock()

	state, ok := m.GetState("myapp")
	require.True(t, ok)
	assert.Equal(t, "deploying", state.Status)
}
