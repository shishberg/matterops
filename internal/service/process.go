package service

import (
	"context"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type ProcessBackend struct {
	cmd        string
	workingDir string
	mu         sync.Mutex
	process    *exec.Cmd
	done       chan struct{} // closed when the process exits
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

	cmd := exec.Command("sh", "-c", p.cmd)
	cmd.Dir = p.workingDir
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}

	p.process = cmd
	done := make(chan struct{})
	p.done = done

	go func() {
		_ = cmd.Wait()
		close(done)
	}()

	return nil
}

func (p *ProcessBackend) Stop(ctx context.Context) error {
	p.mu.Lock()

	if !p.isRunning() {
		p.mu.Unlock()
		return nil
	}

	pgid, err := syscall.Getpgid(p.process.Process.Pid)
	if err != nil {
		p.mu.Unlock()
		return err
	}
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		p.mu.Unlock()
		return err
	}

	done := p.done
	p.process = nil
	p.done = nil
	p.mu.Unlock()

	// Wait for the process to exit, with a fallback force-kill after 5 seconds.
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
		<-done
	}

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
