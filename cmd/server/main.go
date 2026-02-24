package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/autotls"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/acme/autocert"

	"vinr.eu/vanguard/internal/aws"
	"vinr.eu/vanguard/internal/citadel"
	"vinr.eu/vanguard/internal/config"
	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/environment"
	"vinr.eu/vanguard/internal/source"
)

func main() {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Create TokenProvider based on mode
	var githubTokenProvider source.TokenProvider
	if cfg.Mode == "local" {
		githubTokenProvider = func(ctx context.Context) (string, error) {
			return os.Getenv("GITHUB_TOKEN"), nil
		}
	} else {
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
		githubTokenProvider = client.GetGithubAccessToken
	}

	// Load AWS configuration and initialize the Secrets Manager client
	awsCfg, err := aws.LoadServiceConfig(ctx, "SM")
	if err != nil {
		slog.Error("Failed to load AWS config", "error", err)
		os.Exit(1)
	}
	smClient := aws.NewSecretsManagerClient(awsCfg)

	// Load environment manager and Boot the environment
	manager := environment.NewManager(cfg.WorkspaceDir, githubTokenProvider, smClient)
	if err := manager.Boot(ctx, cfg.EnvDefsGitURL, cfg.EnvDefsDir); err != nil {
		slog.Error("Failed to boot engine", "error", err)
		os.Exit(1)
	}

	// Set up the reverse proxy
	router := gin.New()
	setupLogging(router)
	router.Use(gin.Recovery())
	setupReverseProxy(router, manager.GetServices())

	// Variable to hold the local server for graceful shutdown
	var localSrv *http.Server

	if cfg.Mode == "local" {
		localSrv = &http.Server{
			Handler: router,
			Addr:    "0.0.0.0:8080",
		}
		go func() {
			slog.Info("Starting local HTTP server on :8080")
			if err := localSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				slog.Error("Failed to listen", "error", err)
			}
		}()
	} else {
		domains := getDomains(manager.GetServices())
		m := autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(domains...),
			Cache:      autocert.DirCache("/var/www/.cache"),
		}

		go func() {
			slog.Info("Starting AutoTLS server on ports 80 and 443", "domains", domains)
			if err := autotls.RunWithManager(router, &m); err != nil {
				slog.Error("AutoTLS server failed", "error", err)
			}
		}()
	}

	// Wait for the interrupt signal to gracefully shut down the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("Shutdown signal received...")

	ctxTimeout, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shut down the local HTTP server if it was started
	if localSrv != nil {
		if err := localSrv.Shutdown(ctxTimeout); err != nil {
			slog.Error("Failed to shutdown local server", "error", err)
		}
	}

	// Shut down the environment manager
	manager.Shutdown()
	slog.Info("Server exiting")
}

func setupLogging(router *gin.Engine) {
	router.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
		var statusColor, methodColor, resetColor string
		if param.IsOutputColor() {
			statusColor = param.StatusCodeColor()
			methodColor = param.MethodColor()
			resetColor = param.ResetColor()
		}

		if param.Latency > time.Minute {
			param.Latency = param.Latency.Truncate(time.Second)
		}

		host := ""
		if param.Request != nil {
			host = param.Request.Host
		}

		return fmt.Sprintf("[GIN] %v |%s %3d %s| %13v | %15s | %s |%s %-7s %s %#v\n%s",
			param.TimeStamp.Format("2006/01/02 - 15:04:05"),
			statusColor, param.StatusCode, resetColor,
			param.Latency,
			param.ClientIP,
			host,
			methodColor, param.Method, resetColor,
			param.Path,
			param.ErrorMessage,
		)
	}))
}

func getDomains(services map[string]*defs.Service) []string {
	domains := make([]string, 0, len(services))
	for _, svc := range services {
		if svc.IngressHost == nil {
			continue
		}
		domains = append(domains, *svc.IngressHost)
	}
	return domains
}

func setupReverseProxy(router *gin.Engine, services map[string]*defs.Service) {
	proxies := make(map[string]*httputil.ReverseProxy)

	for _, svc := range services {
		if svc.IngressHost == nil {
			continue
		}

		port := fmt.Sprintf("%d", svc.Port)

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
