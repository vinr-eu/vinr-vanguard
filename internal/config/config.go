package config

import (
	"fmt"
	"os"
)

type Config struct {
	Mode             string
	CitadelURL       string
	CitadelAPIKey    string
	EnvDefsGitHubURL string
	EnvDefsDir       string
}

func Load() (*Config, error) {
	mode := os.Getenv("MODE")
	if mode == "" {
		mode = "local"
	}
	if mode != "local" && mode != "server" {
		return nil, fmt.Errorf("MODE must be 'local' or 'server', got %q", mode)
	}

	citadelURL := os.Getenv("CITADEL_URL")
	citadelAPIKey := os.Getenv("CITADEL_API_KEY")

	if mode == "local" && citadelURL == "" {
		citadelURL = "http://localhost:9080"
	}
	if mode != "local" && citadelURL == "" {
		return nil, fmt.Errorf("CITADEL_URL must be set in server mode")
	}
	if mode != "local" && citadelAPIKey == "" {
		return nil, fmt.Errorf("CITADEL_API_KEY must be set")
	}

	envDefsGitHubURL := os.Getenv("ENV_DEFS_GITHUB_URL")
	envDefsDir := os.Getenv("ENV_DEFS_DIR")
	if mode != "local" && envDefsGitHubURL == "" {
		return nil, fmt.Errorf("ENV_DEFS_GITHUB_URL must be set in server mode")
	} else if mode == "local" && envDefsGitHubURL == "" && envDefsDir == "" {
		return nil, fmt.Errorf("ENV_DEFS_GITHUB_URL or ENV_DEFS_DIR must be set in local mode")
	}

	return &Config{
		Mode:             mode,
		CitadelURL:       citadelURL,
		CitadelAPIKey:    citadelAPIKey,
		EnvDefsGitHubURL: envDefsGitHubURL,
		EnvDefsDir:       envDefsDir,
	}, nil
}
