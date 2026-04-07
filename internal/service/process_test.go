package service_test

import (
	"context"
	"testing"
	"time"

	"github.com/shishberg/matterops/internal/service"
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
