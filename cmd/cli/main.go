package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"

	"vinr.eu/vanguard/internal/loader"
)

func main() {
	dirPath := flag.String("dir", "./manifests", "Path to the directory containing JSON manifests")

	debug := flag.Bool("debug", false, "Enable debug logging")

	flag.Parse()

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
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

	fmt.Printf("\n--- Loaded %d Services ---\n", len(store.Services))
	for _, svc := range store.Services {
		fmt.Printf("- %s (Port: %d)\n", svc.Name, svc.Port)
	}
}
