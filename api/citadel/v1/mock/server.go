package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

type Server struct{}

func NewServer() Server {
	return Server{}
}

func (s Server) GetAwsSecretsId(c *gin.Context, id string) {
	envKey := fmt.Sprintf("SECRET_%s", strings.ToUpper(strings.ReplaceAll(id, "-", "_")))
	val, exists := os.LookupEnv(envKey)
	if !exists {
		errResp := ErrorResponse{
			Code:    http.StatusNotFound,
			Message: fmt.Sprintf("Secret not found. Expected environment variable: %s", envKey),
		}
		c.JSON(http.StatusNotFound, errResp)
	}
	resp := GetAwsSecretResponse{
		PlainText: &val,
	}
	c.JSON(http.StatusOK, resp)
}

func (s Server) GetPing(c *gin.Context) {
	version := "1.0.0"
	resp := PingResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   &version,
	}
	c.JSON(http.StatusOK, resp)
}

func (s Server) GetGithubAccessToken(c *gin.Context) {
	accessToken := os.Getenv("GITHUB_ACCESS_TOKEN")
	resp := GetGitHubAccessTokenResponse{
		AccessToken: accessToken,
	}
	c.JSON(http.StatusOK, resp)
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
