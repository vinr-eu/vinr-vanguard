package environment

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/deployment"
	"vinr.eu/vanguard/internal/source"
	"vinr.eu/vanguard/internal/toolchain"
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
			return fmt.Errorf("failed to initialize source: %w", err)
		}
		if err := envSrc.Fetch(ctx, m.workspaceDir); err != nil {
			return fmt.Errorf("failed to fetch env specs: %w", err)
		}
	} else if envDefsDir != "" {
		slog.InfoContext(ctx, "using local env specs", "path", envDefsDir)
		envPath = envDefsDir
	} else {
		return fmt.Errorf("engine error: no source for environment definitions")
	}

	if err := m.defsStore.Load(envPath); err != nil {
		return fmt.Errorf("failed to load defsStore from %s: %w", envPath, err)
	}

	runtimePaths, err := m.ProvisionAll(ctx)
	if err != nil {
		return fmt.Errorf("environment preparation failed: %w", err)
	}

	for _, svc := range m.defsStore.Services {
		key := fmt.Sprintf("%s:%s", svc.Runtime.Engine, svc.Runtime.Version)
		binDir := runtimePaths[key]

		if err := m.deployService(ctx, svc, binDir); err != nil {
			slog.ErrorContext(ctx, "deployment failed to start", "service", svc.Name, "error", err)
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
			return nil, err
		}

		slog.InfoContext(ctx, "pre-provisioning toolchain", "spec", key)
		binDir, err := tc.Provision(ctx, spec.Version)
		if err != nil {
			return nil, fmt.Errorf("failed to provision %s: %w", key, err)
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
		slog.Info("Stopping service", "service", name)
		if err := dep.Stop(); err != nil {
			slog.Error("Failed to stop service", "service", name, "error", err)
		}
	}
}

func (m *Manager) deployService(ctx context.Context, svc *defs.Service, binDir string) error {
	if svc.GitURL == "" {
		return fmt.Errorf("no Git URL for service %s", svc.Name)
	}

	repoPath := filepath.Join(m.workspaceDir, "services", svc.Name)

	src, err := source.New(svc.GitURL, svc.Branch, m.githubTokenProvider)
	if err != nil {
		return fmt.Errorf("failed to initialize source: %w", err)
	}
	if err := src.Fetch(ctx, repoPath); err != nil {
		return fmt.Errorf("failed to fetch source: %w", err)
	}

	dep, err := deployment.New(svc, repoPath, binDir)
	if err != nil {
		return fmt.Errorf("failed to create deployment dep: %w", err)
	}

	if err := dep.Install(ctx); err != nil {
		return fmt.Errorf("install error: %w", err)
	}

	if err := dep.Start(ctx); err != nil {
		return fmt.Errorf("start error: %w", err)
	}

	m.activeDeployments[svc.Name] = dep
	return nil
}
