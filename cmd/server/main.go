package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"vinr.eu/vanguard/internal/citadel"
	"vinr.eu/vanguard/internal/config"
	"vinr.eu/vanguard/internal/engine"
	"vinr.eu/vanguard/internal/logger"
)

func main() {
	logger.InitLogger()
	ctx := context.Background()

	cfg, err := config.Load()
	if err != nil {
		logger.Error(ctx, "Failed to load config", "error", err)
		os.Exit(1)
	}
	citadelClient, err := citadel.NewClient(
		ctx,
		cfg.CitadelURL,
		citadel.WithAPIKey(cfg.CitadelAPIKey),
		citadel.WithTimeout(5*time.Second),
	)
	if err != nil {
		logger.Error(ctx, "Failed to init citadel manager", "error", err)
		os.Exit(1)
	}

	if err := engine.Boot(ctx, cfg, citadelClient); err != nil {
		logger.Error(ctx, "Failed to boot engine", "error", err)
		os.Exit(1)
	}

	router := gin.Default()
	router.Use(logger.Middleware())

	srv := &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:8080",
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error(ctx, "Failed to listen", "error", err)
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
