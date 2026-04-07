package service_test

import (
	"testing"
	"time"

	"github.com/shishberg/matterops/internal/service"
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
