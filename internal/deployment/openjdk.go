package deployment

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

type OpenJDKDeployment struct {
	svc      *defs.Service
	execPath string
	binDir   string
	cmd      *exec.Cmd
	logger   *slog.Logger
}

func NewOpenJDKDeployment(svc *defs.Service, repoPath, binDir string) *OpenJDKDeployment {
	execPath := repoPath
	if svc.Path != "" {
		execPath = filepath.Join(repoPath, svc.Path)
	}

	return &OpenJDKDeployment{
		svc:      svc,
		execPath: execPath,
		binDir:   binDir,
		logger: slog.Default().With("svc", svc.Name, "engine", svc.Runtime.Engine,
			"version", svc.Runtime.Version),
	}
}

func (d *OpenJDKDeployment) Install(ctx context.Context) error {
	manager, args := d.detectManager()
	d.logger.Info("building artifact", "manager", manager)

	cmd := exec.CommandContext(ctx, manager, args...)
	cmd.Dir = d.execPath
	cmd.Env = d.buildEnv()

	if err := d.setupPipes(ctx, cmd); err != nil {
		return err
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s build failed: %w", manager, err)
	}

	return nil
}

func (d *OpenJDKDeployment) Start(ctx context.Context) error {
	if d.svc.RunScript == "" {
		d.logger.Warn("no runScript provided, nothing to start")
		return nil
	}

	args := strings.Fields(d.svc.RunScript)
	commandName := args[0]

	if d.binDir != "" && (commandName == "java" || commandName == "java.exe") {
		commandName = filepath.Join(d.binDir, commandName)
	}

	cmd := exec.CommandContext(ctx, commandName, args[1:]...)
	cmd.Dir = d.execPath
	cmd.Env = d.buildEnv()

	if err := d.setupPipes(ctx, cmd); err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	d.cmd = cmd
	d.logger.Info("process started", "pid", cmd.Process.Pid)
	return nil
}

func (d *OpenJDKDeployment) Stop() error {
	if d.cmd != nil && d.cmd.Process != nil {
		return d.cmd.Process.Signal(os.Interrupt)
	}
	return nil
}

func (d *OpenJDKDeployment) buildEnv() []string {
	env := os.Environ()

	if d.binDir != "" {
		existingPath := os.Getenv("PATH")
		env = append(env, fmt.Sprintf("PATH=%s:%s", d.binDir, existingPath))

		javaHome := filepath.Dir(d.binDir)
		env = append(env, fmt.Sprintf("JAVA_HOME=%s", javaHome))
	}

	for _, v := range d.svc.Variables {
		env = append(env, fmt.Sprintf("%s=%s", v.Name, v.Value))
	}
	return env
}

func (d *OpenJDKDeployment) detectManager() (string, []string) {
	if _, err := os.Stat(filepath.Join(d.execPath, "mvnw")); err == nil {
		return filepath.Join(d.execPath, "mvnw"), []string{"clean", "package", "-DskipTests"}
	}
	if _, err := os.Stat(filepath.Join(d.execPath, "gradlew")); err == nil {
		return filepath.Join(d.execPath, "gradlew"), []string{"build", "-x", "test"}
	}
	if _, err := os.Stat(filepath.Join(d.execPath, "pom.xml")); err == nil {
		return "mvn", []string{"clean", "package", "-DskipTests"}
	}
	return "./gradlew", []string{"build", "-x", "test"}
}

func (d *OpenJDKDeployment) setupPipes(ctx context.Context, cmd *exec.Cmd) error {
	stdout, _ := cmd.StdoutPipe()
	stderr, _ := cmd.StderrPipe()

	go d.logPipe(ctx, stdout, slog.LevelInfo)
	go d.logPipe(ctx, stderr, slog.LevelError)
	return nil
}

func (d *OpenJDKDeployment) logPipe(ctx context.Context, rc io.ReadCloser, level slog.Level) {
	defer rc.Close()
	scanner := bufio.NewScanner(rc)
	for scanner.Scan() {
		d.logger.Log(ctx, level, scanner.Text())
	}
}
