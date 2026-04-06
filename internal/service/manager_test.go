package service_test

import (
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

func TestManager_GetServiceConfig(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()
	cfg, ok := m.GetServiceConfig("testapp")
	require.True(t, ok)
	assert.Equal(t, "testapp", cfg.Name)
	assert.Equal(t, "main", cfg.Branch)
	_, ok = m.GetServiceConfig("nonexistent")
	assert.False(t, ok)
}

func TestManager_FindServiceByRepo(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()
	cfg, ok := m.FindServiceByRepo("github.com/org/testapp", "main")
	require.True(t, ok)
	assert.Equal(t, "testapp", cfg.Name)
	_, ok = m.FindServiceByRepo("github.com/org/testapp", "other-branch")
	assert.False(t, ok)
	_, ok = m.FindServiceByRepo("github.com/org/other", "main")
	assert.False(t, ok)
}

func TestManager_Deploy(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()
	err := m.RequestDeploy("testapp")
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		state, _ := m.GetState("testapp")
		return state.Status != "deploying"
	}, 5*time.Second, 50*time.Millisecond)
	state, _ := m.GetState("testapp")
	assert.Equal(t, "running", state.Status)
	assert.Equal(t, "success", state.LastResult)
	assert.Contains(t, state.LastOutput, "deployed")
}

func TestManager_DeployUnknownService(t *testing.T) {
	m := newTestManager(t)
	defer m.Stop()
	err := m.RequestDeploy("nonexistent")
	assert.Error(t, err)
}

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

func TestManager_BackendSelection(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name   string
		config config.ServiceConfig
	}{
		{
			name: "process backend",
			config: config.ServiceConfig{
				Name: "proc-svc", WorkingDir: dir,
				Process: config.ProcessConfig{Cmd: "sleep 60"},
			},
		},
		{
			name: "service_name selects system backend",
			config: config.ServiceConfig{
				Name: "sys-svc", WorkingDir: dir,
				ServiceName: "myunit",
			},
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
