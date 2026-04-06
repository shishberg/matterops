package service

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
)

type ProcessBackend struct {
	cmd        string
	workingDir string
	mu         sync.Mutex
	process    *exec.Cmd
}

func NewProcessBackend(cmd string, workingDir string) *ProcessBackend {
	return &ProcessBackend{
		cmd:        cmd,
		workingDir: workingDir,
	}
}

func (p *ProcessBackend) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning() {
		return nil
	}

	cmd := exec.CommandContext(ctx, "sh", "-c", p.cmd)
	cmd.Dir = p.workingDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	p.process = cmd

	go func() {
		_ = cmd.Wait()
	}()

	return nil
}

func (p *ProcessBackend) Stop(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.isRunning() {
		return nil
	}

	pgid, err := syscall.Getpgid(p.process.Process.Pid)
	if err != nil {
		return err
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		return err
	}

	p.process = nil
	return nil
}

func (p *ProcessBackend) Restart(ctx context.Context) error {
	if err := p.Stop(ctx); err != nil {
		return err
	}
	return p.Start(ctx)
}

func (p *ProcessBackend) Status(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.isRunning() {
		return "running", nil
	}
	return "stopped", nil
}

func (p *ProcessBackend) isRunning() bool {
	if p.process == nil || p.process.Process == nil {
		return false
	}
	err := p.process.Process.Signal(syscall.Signal(0))
	return err == nil
}
