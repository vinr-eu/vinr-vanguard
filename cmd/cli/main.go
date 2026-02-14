package main

import (
	"context"
	"flag"
	"os"

	"vinr.eu/vanguard/internal/code"
	"vinr.eu/vanguard/internal/loader"
	"vinr.eu/vanguard/internal/logger"
)

func main() {
	logger.InitLogger()
	ctx := context.Background()

	dirPath := flag.String("dir", "./manifests", "Path to the directory containing JSON manifests")
	tokenFlag := flag.String("github-token", "", "GitHub access token")

	flag.Parse()

	githubToken := *tokenFlag
	if githubToken == "" {
		githubToken = os.Getenv("GITHUB_TOKEN")
	}
	if githubToken == "" {
		logger.Warn(ctx, "no GitHub token provided")
		os.Exit(1)
	}

	if info, err := os.Stat(*dirPath); err != nil || !info.IsDir() {
		logger.Error(ctx, "Invalid directory path", "path", *dirPath)
		os.Exit(1)
	}

	logger.Info(ctx, "starting loader", "directory", *dirPath)

	store, err := loader.LoadDir(ctx, *dirPath)
	if err != nil {
		logger.Error(ctx, "Fatal error loading directory", "error", err)
		os.Exit(1)
	}

	for _, svc := range store.Services {
		err := code.Checkout(ctx, svc.GitHubURL, githubToken, "/tmp/"+svc.Name)
		if err != nil {
			logger.Error(ctx, "Failed to checkout service", "service", svc.Name, "error", err)
			os.Exit(1)
		}
	}
}
