package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"vinr.eu/vanguard/internal/citadel"
	"vinr.eu/vanguard/internal/config"
	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/environment"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Load Citadel client
	client, err := citadel.NewClient(
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

	// Load environment manager and Boot the environment
	manager := environment.NewManager(cfg.WorkspaceDir, githubTokenProvider)
	if err := manager.Boot(ctx, cfg.EnvDefsGitURL, cfg.EnvDefsDir); err != nil {
		slog.Error("Failed to boot engine", "error", err)
		os.Exit(1)
	}

	// Set up the reverse proxy
	router := gin.Default()
	setupReverseProxy(router, manager.GetServices())
	srv := &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:80",
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

func setupReverseProxy(router *gin.Engine, services map[string]*defs.Service) {
	proxies := make(map[string]*httputil.ReverseProxy)

	for _, svc := range services {
		if svc.IngressHost == nil {
			continue
		}

		var port string
		for _, v := range svc.Variables {
			if v.Name == "PORT" {
				port = v.Value
				break
			}
		}

		if port == "" {
			slog.Warn("Service has IngressHost but no PORT variable", "service", svc.Name)
			continue
		}

		target, err := url.Parse("http://localhost:" + port)
		if err != nil {
			slog.Error("Failed to parse target URL", "service", svc.Name, "error", err)
			continue
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		host := *svc.IngressHost

		slog.Info("Setting up reverse proxy", "service", svc.Name, "host", host, "port", port)
		proxies[host] = proxy
	}

	if len(proxies) > 0 {
		router.Any("/*proxyPath", func(c *gin.Context) {
			if proxy, ok := proxies[c.Request.Host]; ok {
				proxy.ServeHTTP(c.Writer, c.Request)
				c.Abort()
				return
			}
			c.Next()
		})
	}
}
