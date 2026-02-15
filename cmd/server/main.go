package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"vinr.eu/vanguard/internal/citadel"
	"vinr.eu/vanguard/internal/config"
	"vinr.eu/vanguard/internal/environment"
)

func main() {
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}
	client, err := citadel.NewClient(
		ctx,
		cfg.CitadelURL,
		citadel.WithAPIKey(cfg.CitadelAPIKey),
		citadel.WithTimeout(5*time.Second),
	)
	if err != nil {
		slog.Error("Failed to init citadel manager", "error", err)
		os.Exit(1)
	}

	githubTokenProvider := func(ctx context.Context) (string, error) {
		return client.GetGithubAccessToken(ctx)
	}

	manager := environment.NewManager(cfg.WorkspaceDir, githubTokenProvider)

	if err := manager.Boot(ctx, cfg.EnvDefsGitURL, cfg.EnvDefsDir); err != nil {
		slog.Error("Failed to boot engine", "error", err)
		os.Exit(1)
	}

	router := gin.Default()

	srv := &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:8080",
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("Failed to listen", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Failed to shutdown server", "error", err)
	}
	slog.Info("Server exiting")
}
