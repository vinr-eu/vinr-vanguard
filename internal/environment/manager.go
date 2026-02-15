package environment

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"

	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/service"
	"vinr.eu/vanguard/internal/source"
)

type TokenProvider func(ctx context.Context) (string, error)

type Manager struct {
	workspaceDir     string
	fetchGitHubToken TokenProvider
	defsStore        *defs.Store
	serviceRunners   map[string]*service.Runner
}

func NewManager(workspaceDir string, githubTokenProvider TokenProvider) *Manager {
	return &Manager{
		workspaceDir:     workspaceDir,
		fetchGitHubToken: githubTokenProvider,
		defsStore:        defs.NewStore(),
		serviceRunners:   make(map[string]*service.Runner),
	}
}

func (m *Manager) Boot(ctx context.Context, envDefsGitURL string, envDefsDir string) error {
	var envPath string

	if envDefsGitURL != "" && envDefsDir != "" {
		token, err := m.fetchGitHubToken(ctx)
		if err != nil {
			return fmt.Errorf("failed to get fuel: %w", err)
		}

		envPath = filepath.Join(m.workspaceDir, envDefsDir)
		envSrc := &source.GitHubSource{RepoURL: envDefsGitURL, Token: token}

		slog.InfoContext(ctx, "fetching remote env specs", "url", envDefsGitURL, "into", m.workspaceDir)
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

	for _, svc := range m.defsStore.Services {
		if err := m.deployService(ctx, svc); err != nil {
			slog.ErrorContext(ctx, "piston failed to fire", "service", svc.Name, "error", err)
			continue
		}
	}

	return nil
}

func (m *Manager) deployService(ctx context.Context, svc *defs.Service) error {
	if svc.GitURL == "" {
		return fmt.Errorf("no Git URL for service %s", svc.Name)
	}

	repoPath := filepath.Join(m.workspaceDir, "services", svc.Name)

	token, err := m.fetchGitHubToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to get fuel: %w", err)
	}
	src := &source.GitHubSource{RepoURL: svc.GitURL, Token: token}
	if err := src.Fetch(ctx, repoPath); err != nil {
		return err
	}

	runner := service.NewRunner(svc, repoPath)

	if err := runner.Install(ctx); err != nil {
		return fmt.Errorf("install error: %w", err)
	}

	if err := runner.Start(ctx); err != nil {
		return fmt.Errorf("start error: %w", err)
	}

	m.serviceRunners[svc.Name] = runner
	return nil
}
