package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"vinr.eu/vanguard/internal/github"
	"vinr.eu/vanguard/internal/loader"
	"vinr.eu/vanguard/internal/logger"
	"vinr.eu/vanguard/internal/state"
)

func main() {
	logger.InitLogger()
	ctx := context.Background()
	if err := state.InitCitadel(5 * time.Second); err != nil {
		logger.Error(ctx, "Failed to init state", "error", err)
		os.Exit(1)
	}
	router := gin.Default()
	router.Use(logger.Middleware())

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	envDefsGitHubUrl := os.Getenv("ENV_DEFS_GITHUB_URL")
	envDefsDir := os.Getenv("ENV_DEFS_DIR")
	if env != "development" && envDefsGitHubUrl == "" {
		logger.Error(ctx, "Production mode requires ENV_DEFS_GITHUB_URL to be set")
		os.Exit(1)
	} else if env == "development" && envDefsGitHubUrl == "" && envDefsDir == "" {
		logger.Error(ctx, "Development mode requires either ENV_DEFS_GITHUB_URL or ENV_DEFS_DIR to be set")
		os.Exit(1)
	}

	// Clean up code directory before checkouts
	if err := os.RemoveAll("/tmp/code"); err != nil {
		logger.Warn(ctx, "Failed to clean up /tmp/code", "error", err)
	}

	loadPath := envDefsDir
	if envDefsGitHubUrl != "" {
		repoName, err := github.GetRepoName(envDefsGitHubUrl)
		if err != nil {
			logger.Error(ctx, "Failed to get repo name", "error", err)
			os.Exit(1)
		}
		repoPath := filepath.Join("/tmp/code", repoName)
		if err := github.Checkout(ctx, envDefsGitHubUrl, repoPath); err != nil {
			logger.Error(ctx, "Failed to checkout env defs", "error", err)
			os.Exit(1)
		}
		loadPath = filepath.Join(repoPath, envDefsDir)
	}

	store, err := loader.LoadDir(ctx, loadPath)
	if err != nil {
		logger.Error(ctx, "Failed to load directory", "path", loadPath, "error", err)
		os.Exit(1)
	}

	var runners []*loader.ServiceRunner
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
		if err := github.Checkout(ctx, svc.GitHubURL, repoPath); err != nil {
			logger.Warn(ctx, "Failed to checkout service", "service", svc.Name, "error", err)
			continue
		}

		runner := loader.NewServiceRunner(svc, repoPath)
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

	srv := &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:8080",
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "Failed to listen", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	logger.Info(ctx, "Shutdown Server ...")

	ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctxShutdown); err != nil {
		logger.Error(ctx, "Failed to shutdown server", "error", err)
	}

	for _, r := range runners {
		if r.Cmd != nil && r.Cmd.Process != nil {
			logger.Info(ctx, "Stopping service", "name", r.Service.Name)
			// CommandContext will already kill the process if we cancel the context used to start it,
			// but we didn't save the cancel func for each runner's Start call.
			// Actually, Start uses the main ctx, which is Background() in my implementation.
			// Let's use SIGTERM for clean shutdown of children.
			r.Cmd.Process.Signal(syscall.SIGTERM)
		}
	}
	logger.Info(ctx, "Server exiting")
}
