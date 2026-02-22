package config

import (
	"errors"
	"fmt"
	"os"

	"vinr.eu/vanguard/internal/errs" // Importing your new utility
)

var (
	ErrInvalidMode    = errors.New("config: MODE must be 'local' or 'server'")
	ErrMissingCitadel = errors.New("config: CITADEL_URL or API_KEY missing for server mode")
	ErrMissingEnvDefs = errors.New("config: ENV_DEFS configuration incomplete")
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
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	switch c.Mode {
	case "local":
		if c.CitadelURL == "" {
			c.CitadelURL = "http://localhost:9080"
		}
		if c.EnvDefsGitURL == "" && c.EnvDefsDir == "" {
			return ErrMissingEnvDefs
		}
	case "server":
		if c.CitadelURL == "" || c.CitadelAPIKey == "" {
			return ErrMissingCitadel
		}
		if c.EnvDefsGitURL == "" {
			return ErrMissingEnvDefs
		}
	default:
		return errs.WrapMsg(ErrInvalidMode, "got "+c.Mode)
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
