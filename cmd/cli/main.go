package main

import (
	"flag"
	"log/slog"
	"os"

	"vinr.eu/vanguard/internal/code"
	"vinr.eu/vanguard/internal/loader"
)

func main() {
	dirPath := flag.String("dir", "./manifests", "Path to the directory containing JSON manifests")
	debug := flag.Bool("debug", false, "Enable debug logging")
	tokenFlag := flag.String("github-token", "", "GitHub access token")

	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	githubToken := *tokenFlag
	if githubToken == "" {
		githubToken = os.Getenv("GITHUB_TOKEN")
	}
	if githubToken == "" {
		slog.Warn("no GitHub token provided")
		os.Exit(1)
	}

	opts := &slog.HandlerOptions{Level: logLevel}
	logger := slog.New(slog.NewTextHandler(os.Stdout, opts))
	slog.SetDefault(logger)

	if info, err := os.Stat(*dirPath); err != nil || !info.IsDir() {
		slog.Error("invalid directory path", "path", *dirPath)
		os.Exit(1)
	}

	slog.Info("starting loader", "directory", *dirPath)

	store, err := loader.LoadDir(*dirPath)
	if err != nil {
		slog.Error("fatal error loading directory", "error", err)
		os.Exit(1)
	}

	for _, svc := range store.Services {
		err := code.Checkout(svc.GitHubURL, githubToken, "/tmp/"+svc.Name)
		if err != nil {
			slog.Error("failed to checkout service", "service", svc.Name, "error", err)
			os.Exit(1)
		}
	}
}
