package loader

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/internal/defs/v1"
	"vinr.eu/vanguard/internal/logger"
)

type ServiceRunner struct {
	Service *v1.Service
	Path    string
	Cmd     *exec.Cmd
}

func NewServiceRunner(svc *v1.Service, repoPath string) *ServiceRunner {
	execPath := repoPath
	if svc.Path != nil {
		execPath = filepath.Join(repoPath, *svc.Path)
	}

	return &ServiceRunner{
		Service: svc,
		Path:    execPath,
	}
}

func (r *ServiceRunner) Install(ctx context.Context) error {
	var cmdName string
	if _, err := os.Stat(filepath.Join(r.Path, "pnpm-lock.yaml")); err == nil {
		cmdName = "pnpm"
	} else if _, err := os.Stat(filepath.Join(r.Path, "yarn.lock")); err == nil {
		cmdName = "yarn"
	} else {
		cmdName = "npm"
	}

	logger.Info(ctx, "Running installation", "service", r.Service.Name, "manager", cmdName)

	cmd := exec.CommandContext(ctx, cmdName, "install")
	cmd.Dir = r.Path
	cmd.Env = os.Environ()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe for install: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe for install: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start %s install for %s: %w", cmdName, r.Service.Name, err)
	}

	go r.logPipe(stdout, logger.Info)
	go r.logPipe(stderr, logger.Error)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("%s install failed for %s: %w", cmdName, r.Service.Name, err)
	}

	logger.Info(ctx, "Installation completed", "service", r.Service.Name)
	return nil
}

func (r *ServiceRunner) Start(ctx context.Context) error {
	if r.Service.RunScript == "" {
		logger.Info(ctx, "No runScript provided, skipping start", "service", r.Service.Name)
		return nil
	}
	args := strings.Fields(r.Service.RunScript)

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = r.Path

	// Pass environment variables
	cmd.Env = os.Environ()
	for _, v := range r.Service.Variables {
		if v.Value != nil {
			cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", v.Name, *v.Value))
		}
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start service %s: %w", r.Service.Name, err)
	}

	r.Cmd = cmd

	go r.logPipe(stdout, logger.Info)
	go r.logPipe(stderr, logger.Error)

	logger.Info(ctx, "Service started", "name", r.Service.Name, "path", r.Path, "pid", cmd.Process.Pid)

	return nil
}

func (r *ServiceRunner) logPipe(pipe io.ReadCloser, logFunc func(context.Context, string, ...any)) {
	defer pipe.Close()
	scanner := bufio.NewScanner(pipe)
	ctx := context.WithValue(context.Background(), "app", r.Service.Name)
	for scanner.Scan() {
		logFunc(ctx, scanner.Text())
	}
}

func (r *ServiceRunner) Wait() error {
	if r.Cmd == nil {
		return nil
	}
	return r.Cmd.Wait()
}
