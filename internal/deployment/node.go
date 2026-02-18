package deployment

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrInstallFailed = errors.New("node: install failed")
	ErrStartFailed   = errors.New("node: start failed")
	ErrPipeFailed    = errors.New("node: pipe setup failed")
)

type NodeDeployment struct {
	svc      *defs.Service
	execPath string
	binDir   string
	cmd      *exec.Cmd
	logger   *slog.Logger
}

func NewNodeDeployment(svc *defs.Service, repoPath, binDir string) *NodeDeployment {
	execPath := repoPath
	if svc.Path != "" {
		execPath = filepath.Join(repoPath, svc.Path)
	}
	return &NodeDeployment{
		svc:      svc,
		execPath: execPath,
		binDir:   binDir,
		logger:   slog.Default().With("svc", svc.Name, "engine", svc.Runtime.Engine, "version", svc.Runtime.Version),
	}
}

func (d *NodeDeployment) Install(ctx context.Context) error {
	manager := d.detectManager()
	d.logger.Info("installing dependencies", "manager", manager)
	cmd := exec.CommandContext(ctx, manager, "install")
	cmd.Dir = d.execPath
	if err := d.setupPipes(ctx, cmd); err != nil {
		return errs.Wrap(ErrPipeFailed, err)
	}
	if err := cmd.Run(); err != nil {
		return errs.WrapMsg(ErrInstallFailed, manager, err)
	}
	return nil
}

func (d *NodeDeployment) Start(ctx context.Context) error {
	if d.svc.RunScript == "" {
		d.logger.Warn("no runScript provided, nothing to start")
		return nil
	}
	args := strings.Fields(d.svc.RunScript)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = d.execPath
	cmd.Env = d.buildEnv()
	if err := d.setupPipes(ctx, cmd); err != nil {
		return errs.Wrap(ErrPipeFailed, err)
	}
	if err := cmd.Start(); err != nil {
		return errs.Wrap(ErrStartFailed, err)
	}
	d.cmd = cmd
	d.logger.Info("process started", "pid", cmd.Process.Pid)
	return nil
}

func (d *NodeDeployment) Stop() error {
	if d.cmd != nil && d.cmd.Process != nil {
		return d.cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

func (d *NodeDeployment) buildEnv() []string {
	env := os.Environ()
	if d.binDir != "" {
		existingPath := os.Getenv("PATH")
		env = append(env, fmt.Sprintf("PATH=%s:%s", d.binDir, existingPath))
	}
	for _, v := range d.svc.Variables {
		env = append(env, fmt.Sprintf("%s=%s", v.Name, *v.Value))
	}
	return env
}

func (d *NodeDeployment) detectManager() string {
	checks := []struct{ file, name string }{
		{"pnpm-lock.yaml", "pnpm"},
		{"yarn.lock", "yarn"},
		{"package-lock.json", "npm"},
	}
	for _, m := range checks {
		if _, err := os.Stat(filepath.Join(d.execPath, m.file)); err == nil {
			return m.name
		}
	}
	return "npm"
}

func (d *NodeDeployment) setupPipes(ctx context.Context, cmd *exec.Cmd) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	go d.logPipe(ctx, stdout, slog.LevelInfo)
	go d.logPipe(ctx, stderr, slog.LevelError)
	return nil
}

func (d *NodeDeployment) logPipe(ctx context.Context, rc io.ReadCloser, level slog.Level) {
	defer rc.Close()
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		d.logger.Log(ctx, level, scanner.Text())
	}
}
