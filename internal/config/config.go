package config

import (
	"fmt"
	"os"
)

type Config struct {
	Mode          string
	WorkspaceDir  string
	CitadelURL    string
	CitadelAPIKey string
	EnvDefsGitURL string
	EnvDefsDir    string
}

func Load() (*Config, error) {
	cfg := &Config{
		Mode:          getEnv("MODE", "local"),
		WorkspaceDir:  getEnv("WORKSPACE_DIR", "/tmp"),
		CitadelURL:    os.Getenv("CITADEL_URL"),
		CitadelAPIKey: os.Getenv("CITADEL_API_KEY"),
		EnvDefsGitURL: os.Getenv("ENV_DEFS_GIT_URL"),
		EnvDefsDir:    os.Getenv("ENV_DEFS_DIR"),
	}

	if err := cfg.applyDefaultsAndValidate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (c *Config) applyDefaultsAndValidate() error {
	if c.Mode != "local" && c.Mode != "server" {
		return fmt.Errorf("MODE must be 'local' or 'server', got %q", c.Mode)
	}

	if c.Mode == "local" && c.CitadelURL == "" {
		c.CitadelURL = "http://localhost:9080"
	}

	if c.Mode == "server" {
		if c.CitadelURL == "" {
			return fmt.Errorf("CITADEL_URL must be set in server mode")
		}
		if c.CitadelAPIKey == "" {
			return fmt.Errorf("CITADEL_API_KEY must be set in server mode")
		}
		if c.EnvDefsGitURL == "" {
			return fmt.Errorf("ENV_DEFS_GITHUB_URL must be set in server mode")
		}
	}

	if c.Mode == "local" && c.EnvDefsGitURL == "" && c.EnvDefsDir == "" {
		return fmt.Errorf("either ENV_DEFS_GITHUB_URL or ENV_DEFS_DIR must be set in local mode")
	}

	return nil
}

func (c *Config) String() string {
	return fmt.Sprintf(
		"Mode=%s WorkspaceDir=%s CitadelURL=%s EnvDefsGitURL=%s EnvDefsDir=%s",
		c.Mode, c.WorkspaceDir, c.CitadelURL, c.EnvDefsGitURL, c.EnvDefsDir,
	)
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
