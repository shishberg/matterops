package service

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// SystemctlBackend manages a systemd service unit via systemctl.
type SystemctlBackend struct {
	unit string
}

// NewSystemctlBackend returns a Backend that controls the given systemd unit.
func NewSystemctlBackend(unit string) *SystemctlBackend {
	return &SystemctlBackend{unit: unit}
}

func (s *SystemctlBackend) Start(ctx context.Context) error {
	return exec.CommandContext(ctx, "systemctl", "start", s.unit).Run()
}

func (s *SystemctlBackend) Stop(ctx context.Context) error {
	return exec.CommandContext(ctx, "systemctl", "stop", s.unit).Run()
}

func (s *SystemctlBackend) Restart(ctx context.Context) error {
	return exec.CommandContext(ctx, "systemctl", "restart", s.unit).Run()
}

// Status returns "running", "stopped", or "failed" based on systemctl is-active output.
func (s *SystemctlBackend) Status(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "systemctl", "is-active", s.unit).Output()
	state := strings.TrimSpace(string(out))

	if err != nil {
		// systemctl is-active exits non-zero for inactive/failed units; still return a
		// mapped status when the output is a recognised state word.
		switch state {
		case "inactive", "dead":
			return "stopped", nil
		case "failed":
			return "failed", nil
		}
		return "", fmt.Errorf("systemctl is-active %s: %w", s.unit, err)
	}

	if state == "active" {
		return "running", nil
	}
	return state, nil
}
