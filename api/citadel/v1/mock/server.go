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

func NewServer() Server {
	return Server{}
}

func (s Server) Ping(ctx *gin.Context) {
	version := "1.0.0"
	resp := PingResponse{
		Status:    "ok",
		Timestamp: time.Now(),
		Version:   &version,
	}
	ctx.JSON(http.StatusOK, resp)
}

func (s Server) GetGitHubAccessToken(ctx *gin.Context) {
	accessToken := os.Getenv("GITHUB_ACCESS_TOKEN")
	resp := GetGitHubAccessTokenResponse{
		AccessToken: accessToken,
	}
	ctx.JSON(http.StatusOK, resp)
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
