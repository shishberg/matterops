package service

import (
	"sync"
	"time"
)

type pendingConfirmation struct {
	branch    string
	createdAt time.Time
}

// ConfirmationTracker tracks pending deploy confirmations with expiry.
type ConfirmationTracker struct {
	mu      sync.Mutex
	pending map[string]pendingConfirmation
	timeout time.Duration
}

// NewConfirmationTracker creates a ConfirmationTracker with the given expiry timeout.
func NewConfirmationTracker(timeout time.Duration) *ConfirmationTracker {
	return &ConfirmationTracker{
		pending: make(map[string]pendingConfirmation),
		timeout: timeout,
	}
}

// AddPending registers a pending confirmation for a service. Overwrites any existing entry.
func (ct *ConfirmationTracker) AddPending(serviceName string, branch string) {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	ct.pending[serviceName] = pendingConfirmation{
		branch:    branch,
		createdAt: time.Now(),
	}
}

// IsPending reports whether there is a non-expired pending confirmation for the service.
// If the entry has expired it is deleted and false is returned.
func (ct *ConfirmationTracker) IsPending(serviceName string) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	entry, ok := ct.pending[serviceName]
	if !ok {
		return false
	}
	if time.Since(entry.createdAt) > ct.timeout {
		delete(ct.pending, serviceName)
		return false
	}
	return true
}

// Confirm removes the pending confirmation for a service and returns true if it existed
// and had not expired. Returns false if no pending confirmation exists or it has expired.
func (ct *ConfirmationTracker) Confirm(serviceName string) bool {
	ct.mu.Lock()
	defer ct.mu.Unlock()
	entry, ok := ct.pending[serviceName]
	if !ok {
		return false
	}
	if time.Since(entry.createdAt) > ct.timeout {
		delete(ct.pending, serviceName)
		return false
	}
	delete(ct.pending, serviceName)
	return true
}
