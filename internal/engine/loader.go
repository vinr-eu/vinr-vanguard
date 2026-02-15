package engine

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"vinr.eu/vanguard/internal/citadel"
	"vinr.eu/vanguard/internal/config"
)

func Boot(ctx context.Context, cfg *config.Config, citadelClient *citadel.Client) error {
	// Clean up the code directory before checkouts
	if err := os.RemoveAll("/tmp/code"); err != nil {
		slog.Warn("Failed to clean up /tmp/code", "error", err)
	}

	loadPath := cfg.EnvDefsDir
	if cfg.EnvDefsGitHubURL != "" {
		repoName, err := getRepoName(cfg.EnvDefsGitHubURL)
		if err != nil {
			return fmt.Errorf("failed to get repo name: %w", err)
		}
		repoPath := filepath.Join("/tmp/code", repoName)
		if err := cloneRepository(ctx, cfg.EnvDefsGitHubURL, repoPath, citadelClient); err != nil {
			return fmt.Errorf("failed to checkout env defs: %w", err)
		}
		loadPath = filepath.Join(repoPath, cfg.EnvDefsDir)
	}

	store, err := LoadDir(loadPath)
	if err != nil {
		return fmt.Errorf("failed to load directory %s: %w", loadPath, err)
	}

	var runners []*ServiceRunner
	for _, svc := range store.Services {
		if svc.GitHubURL == "" {
			continue
		}
		repoName, err := getRepoName(svc.GitHubURL)
		if err != nil {
			slog.Warn("Failed to get repo name for service", "service", svc.Name, "error", err)
			continue
		}
		repoPath := filepath.Join("/tmp/code", repoName)
		if err := cloneRepository(ctx, svc.GitHubURL, repoPath, citadelClient); err != nil {
			slog.Warn("Failed to checkout service", "service", svc.Name, "error", err)
			continue
		}

		runner := NewServiceRunner(svc, repoPath)
		if err := runner.Install(ctx); err != nil {
			slog.Error("Failed to install dependencies", "service", svc.Name, "error", err)
			continue
		}

		if err := runner.Start(ctx); err != nil {
			slog.Error("Failed to start service", "service", svc.Name, "error", err)
		} else if runner.Cmd != nil {
			runners = append(runners, runner)
		}
	}
	return nil
}
