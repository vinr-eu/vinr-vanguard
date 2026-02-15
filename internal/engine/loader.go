package engine

import (
	"context"
	"os"
	"path/filepath"

	"vinr.eu/vanguard/internal/citadel"
	"vinr.eu/vanguard/internal/config"
	"vinr.eu/vanguard/internal/github"
	"vinr.eu/vanguard/internal/logger"
)

func Boot(ctx context.Context, cfg *config.Config, citadelClient *citadel.Client) {
	// Clean up the code directory before checkouts
	if err := os.RemoveAll("/tmp/code"); err != nil {
		logger.Warn(ctx, "Failed to clean up /tmp/code", "error", err)
	}

	loadPath := cfg.EnvDefsDir
	if cfg.EnvDefsGitHubURL != "" {
		repoName, err := github.GetRepoName(cfg.EnvDefsGitHubURL)
		if err != nil {
			logger.Error(ctx, "Failed to get repo name", "error", err)
			os.Exit(1)
		}
		repoPath := filepath.Join("/tmp/code", repoName)
		if err := github.Checkout(ctx, cfg.EnvDefsGitHubURL, repoPath, citadelClient); err != nil {
			logger.Error(ctx, "Failed to checkout env defs", "error", err)
			os.Exit(1)
		}
		loadPath = filepath.Join(repoPath, cfg.EnvDefsDir)
	}

	store, err := LoadDir(ctx, loadPath)
	if err != nil {
		logger.Error(ctx, "Failed to load directory", "path", loadPath, "error", err)
		os.Exit(1)
	}

	var runners []*ServiceRunner
	for _, svc := range store.Services {
		if svc.GitHubURL == "" {
			continue
		}
		repoName, err := github.GetRepoName(svc.GitHubURL)
		if err != nil {
			logger.Warn(ctx, "Failed to get repo name for service", "service", svc.Name, "error", err)
			continue
		}
		repoPath := filepath.Join("/tmp/code", repoName)
		if err := github.Checkout(ctx, svc.GitHubURL, repoPath, citadelClient); err != nil {
			logger.Warn(ctx, "Failed to checkout service", "service", svc.Name, "error", err)
			continue
		}

		runner := NewServiceRunner(svc, repoPath)
		if err := runner.Install(ctx); err != nil {
			logger.Error(ctx, "Failed to install dependencies", "service", svc.Name, "error", err)
			continue
		}

		if err := runner.Start(ctx); err != nil {
			logger.Error(ctx, "Failed to start service", "service", svc.Name, "error", err)
		} else if runner.Cmd != nil {
			runners = append(runners, runner)
		}
	}
}
