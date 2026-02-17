package environment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/deployment"
	"vinr.eu/vanguard/internal/errs"
	"vinr.eu/vanguard/internal/source"
	"vinr.eu/vanguard/internal/toolchain"
)

var (
	ErrBootFailed      = errors.New("environment: boot failed")
	ErrNoSource        = errors.New("environment: no source for definitions")
	ErrProvisionFailed = errors.New("environment: provisioning failed")
	ErrDeployFailed    = errors.New("environment: service deployment failed")
)

type Manager struct {
	workspaceDir        string
	githubTokenProvider source.TokenProvider
	defsStore           *defs.Store
	activeDeployments   map[string]deployment.Deployment
}

func NewManager(workspaceDir string, githubTokenProvider source.TokenProvider) *Manager {
	return &Manager{
		workspaceDir:        workspaceDir,
		githubTokenProvider: githubTokenProvider,
		defsStore:           defs.NewStore(),
		activeDeployments:   make(map[string]deployment.Deployment),
	}
}

func (m *Manager) Boot(ctx context.Context, envDefsGitURL string, envDefsDir string) error {
	var envPath string
	if envDefsGitURL != "" && envDefsDir != "" {
		envPath = filepath.Join(m.workspaceDir, envDefsDir)
		envSrc, err := source.New(envDefsGitURL, "main", m.githubTokenProvider)
		if err != nil {
			return errs.WrapMsg(ErrBootFailed, "source init", err)
		}
		if err := envSrc.Fetch(ctx, m.workspaceDir); err != nil {
			return errs.WrapMsg(ErrBootFailed, "fetch specs", err)
		}
	} else if envDefsDir != "" {
		slog.InfoContext(ctx, "using local env specs", "path", envDefsDir)
		envPath = envDefsDir
	} else {
		return ErrNoSource
	}
	if err := m.defsStore.Load(envPath); err != nil {
		return errs.WrapMsg(ErrBootFailed, "store load", err)
	}
	runtimePaths, err := m.ProvisionAll(ctx)
	if err != nil {
		return errs.Wrap(ErrProvisionFailed, err)
	}
	for _, svc := range m.defsStore.Services {
		key := fmt.Sprintf("%s:%s", svc.Runtime.Engine, svc.Runtime.Version)
		binDir := runtimePaths[key]
		if err := m.deployService(ctx, svc, binDir); err != nil {
			slog.ErrorContext(ctx, "deployment failed", "service", svc.Name, "error", err)
			continue
		}
	}
	return nil
}

func (m *Manager) ProvisionAll(ctx context.Context) (map[string]string, error) {
	required := make(map[string]defs.RuntimeSpec)
	for _, svc := range m.defsStore.Services {
		key := fmt.Sprintf("%s:%s", svc.Runtime.Engine, svc.Runtime.Version)
		required[key] = svc.Runtime
	}
	slog.InfoContext(ctx, "resolving runtimes", "count", len(required))
	results := make(map[string]string)
	for key, spec := range required {
		tc, err := toolchain.New(spec.Engine, m.workspaceDir)
		if err != nil {
			return nil, errs.WrapMsg(ErrProvisionFailed, key, err)
		}
		slog.InfoContext(ctx, "provisioning toolchain", "spec", key)
		binDir, err := tc.Provision(ctx, spec.Version)
		if err != nil {
			return nil, errs.WrapMsg(ErrProvisionFailed, key, err)
		}
		results[key] = binDir
	}
	return results, nil
}

func (m *Manager) GetServices() map[string]*defs.Service {
	return m.defsStore.Services
}

func (m *Manager) Shutdown() {
	for name, dep := range m.activeDeployments {
		slog.Info("stopping service", "service", name)
		if err := dep.Stop(); err != nil {
			slog.Error("shutdown error", "service", name, "error", err)
		}
	}
}

func (m *Manager) deployService(ctx context.Context, svc *defs.Service, binDir string) error {
	if svc.GitURL == "" {
		return errs.WrapMsg(ErrDeployFailed, "no git url: "+svc.Name, nil)
	}
	repoPath := filepath.Join(m.workspaceDir, "services", svc.Name)
	src, err := source.New(svc.GitURL, svc.Branch, m.githubTokenProvider)
	if err != nil {
		return errs.WrapMsg(ErrDeployFailed, "source init: "+svc.Name, err)
	}
	if err := src.Fetch(ctx, repoPath); err != nil {
		return errs.WrapMsg(ErrDeployFailed, "fetch: "+svc.Name, err)
	}
	dep, err := deployment.New(svc, repoPath, binDir)
	if err != nil {
		return errs.WrapMsg(ErrDeployFailed, "dep init: "+svc.Name, err)
	}
	if err := dep.Install(ctx); err != nil {
		return errs.WrapMsg(ErrDeployFailed, "install: "+svc.Name, err)
	}
	if err := dep.Start(ctx); err != nil {
		return errs.WrapMsg(ErrDeployFailed, "start: "+svc.Name, err)
	}
	m.activeDeployments[svc.Name] = dep
	return nil
}
