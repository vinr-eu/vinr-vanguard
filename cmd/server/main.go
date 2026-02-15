package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
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
		}
	}

	citadelUrl, err := url.Parse("http://localhost:9080")
	if err != nil {
		logger.Error(ctx, "Failed to parse upstream URL", "error", err)
		os.Exit(1)
	}
	citadelService := httputil.NewSingleHostReverseProxy(citadelUrl)

	router.Any("/citadel/*any", func(c *gin.Context) {
		c.Request.URL.Path = c.Param("any")
		citadelService.ServeHTTP(c.Writer, c.Request)
	})

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

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error(ctx, "Failed to shutdown server", "error", err)
	}
	logger.Info(ctx, "Server exiting")
}
