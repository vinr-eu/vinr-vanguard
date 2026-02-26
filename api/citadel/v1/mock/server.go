package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct{}

func (s Server) GetNodeIdGetConfig(c *gin.Context, _ string) {
	ingressHost := "vinr.local"
	runScript := "npm run dev"
	port := "3000"
	resp := GetNodeConfigResponse{
		Type: Vanguard,
		ServiceDeployments: []ServiceDeployment{
			{
				Kind:        "service",
				DefVersion:  "v1",
				Name:        "next-js-app",
				GitUrl:      "https://github.com/richy-vinr/next-js-app",
				Branch:      "main",
				Port:        3000,
				IngressHost: &ingressHost,
				RunScript:   &runScript,
				Runtime: struct {
					Engine  string `json:"engine"`
					Version string `json:"version"`
				}{
					Engine:  "node",
					Version: "24.13.1",
				},
				Variables: &[]EnvironmentVariable{
					{
						Name:  "PORT",
						Value: &port,
					},
				},
			},
		},
	}
	c.JSON(http.StatusOK, resp)
}

func (s Server) GetNodeIdPing(c *gin.Context, _ string) {
	version := "1.0.0"
	resp := GetNodePingResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   &version,
	}
	c.JSON(http.StatusOK, resp)
}

func (s Server) GetGithubAccessToken(c *gin.Context) {
	accessToken := os.Getenv("GITHUB_TOKEN")
	resp := GetGitHubAccessTokenResponse{
		AccessToken: accessToken,
	}
	c.JSON(http.StatusOK, resp)
}

func NewServer() Server {
	return Server{}
}

func main() {
	router := gin.Default()
	server := NewServer()
	RegisterHandlers(router, server)
	srv := &http.Server{
		Handler: router,
		Addr:    "0.0.0.0:9080",
	}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Println("Server Shutdown:", err)
	}
	log.Println("Server exiting")
}
