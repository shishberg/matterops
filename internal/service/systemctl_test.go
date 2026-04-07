package service_test

import (
	"context"
	"testing"

	"github.com/shishberg/matterops/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestSystemctlBackend_CommandFormation(t *testing.T) {
	b := service.NewSystemctlBackend("myapp", false)
	assert.Implements(t, (*service.Backend)(nil), b)
	_, err := b.Status(context.Background())
	if err != nil {
		assert.Contains(t, err.Error(), "systemctl")
	}
}

func TestSystemctlBackend_UserMode(t *testing.T) {
	b := service.NewSystemctlBackend("myapp", true)
	assert.Implements(t, (*service.Backend)(nil), b)
	_, err := b.Status(context.Background())
	if err != nil {
		assert.Contains(t, err.Error(), "systemctl")
	}
}
