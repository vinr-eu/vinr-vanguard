package service

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/internal/defs"
)

type Runner struct {
	Service *defs.Service
	Path    string
	Cmd     *exec.Cmd
	logger  *slog.Logger
}

func NewRunner(svc *defs.Service, repoPath string) *Runner {
	execPath := repoPath
	if svc.Path != "" {
		execPath = filepath.Join(repoPath, svc.Path)
	}

	return &Runner{
		Service: svc,
		Path:    execPath,
		logger:  slog.Default().With("service", svc.Name),
	}
}

func (r *Runner) Install(ctx context.Context) error {
	cmdName := r.detectManager()
	r.logger.Info("running installation", "manager", cmdName)

	cmd := exec.CommandContext(ctx, cmdName, "install")
	cmd.Dir = r.Path

	if err := r.setupPipes(ctx, cmd); err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s install failed: %w", cmdName, err)
	}

	r.logger.Info("installation completed")
	return nil
}

func (r *Runner) Start(ctx context.Context) error {
	if r.Service.RunScript == "" {
		r.logger.Info("no runScript provided, skipping")
		return nil
	}

	args := strings.Fields(r.Service.RunScript)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = r.Path

	cmd.Env = r.buildEnv()

	if err := r.setupPipes(ctx, cmd); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start: %w", err)
	}

	r.Cmd = cmd
	r.logger.Info("service started", "pid", cmd.Process.Pid)

	return nil
}

func (r *Runner) buildEnv() []string {
	env := os.Environ()
	for _, v := range r.Service.Variables {
		env = append(env, fmt.Sprintf("%s=%s", v.Name, *v.Value))
	}
	return env
}

func (r *Runner) setupPipes(ctx context.Context, cmd *exec.Cmd) error {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	go r.logPipe(ctx, stdout, slog.LevelInfo)
	go r.logPipe(ctx, stderr, slog.LevelError)
	return nil
}

func (r *Runner) logPipe(ctx context.Context, rc io.ReadCloser, level slog.Level) {
	defer func() {
		if err := rc.Close(); err != nil {
			r.logger.WarnContext(ctx, "failed to close log pipe", "error", err)
		}
	}()

	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		r.logger.Log(ctx, level, scanner.Text())
	}
}

func (r *Runner) detectManager() string {
	for _, m := range []struct{ file, name string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
	} {
		if _, err := os.Stat(filepath.Join(r.Path, m.file)); err == nil {
			return m.name
		}
	}
	return "npm"
}
