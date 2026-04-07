package service_test

import (
	"testing"

	"github.com/shishberg/matterops/internal/service"
	"github.com/stretchr/testify/assert"
)

func TestLaunchctlBackend_Interface(t *testing.T) {
	b := service.NewLaunchctlBackend("com.example.myapp")
	assert.Implements(t, (*service.Backend)(nil), b)
}
