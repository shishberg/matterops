package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// LaunchctlBackend manages macOS services via launchctl.
type LaunchctlBackend struct {
	label string
}

// NewLaunchctlBackend returns a Backend that controls the given launchd service label.
func NewLaunchctlBackend(label string) *LaunchctlBackend {
	return &LaunchctlBackend{label: label}
}

// Start starts the service.
func (b *LaunchctlBackend) Start(ctx context.Context) error {
	return exec.CommandContext(ctx, "launchctl", "start", b.label).Run()
}

// Stop stops the service.
func (b *LaunchctlBackend) Stop(ctx context.Context) error {
	return exec.CommandContext(ctx, "launchctl", "stop", b.label).Run()
}

// Restart stops then starts the service.
func (b *LaunchctlBackend) Restart(ctx context.Context) error {
	if err := b.Stop(ctx); err != nil {
		return fmt.Errorf("launchctl restart stop: %w", err)
	}
	if err := b.Start(ctx); err != nil {
		return fmt.Errorf("launchctl restart start: %w", err)
	}
	return nil
}

// Status returns "running" when the label appears in `launchctl list` output,
// and "stopped" otherwise.
func (b *LaunchctlBackend) Status(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "launchctl", "list").Output()
	if err != nil {
		return "", fmt.Errorf("launchctl list: %w", err)
	}
	if strings.Contains(string(out), b.label) {
		return "running", nil
	}
	return "stopped", nil
}
