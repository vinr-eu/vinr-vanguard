package github

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	ghttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	citadel "vinr.eu/vanguard/api/citadel/v1"
	"vinr.eu/vanguard/internal/logger"
	"vinr.eu/vanguard/internal/state"
)

func Checkout(ctx context.Context, repoURL, destPath string) error {
	if !state.GetCitadel().IsAlive {
		return fmt.Errorf("citadel should be alive to clone repositories")
	}
	logger.Info(ctx, "Cloning repository", "url", repoURL, "destination", destPath)

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	citadelURL := os.Getenv("CITADEL_URL")
	if env == "development" && citadelURL == "" {
		citadelURL = "http://localhost:9080"
	}
	apiKey := os.Getenv("API_KEY")
	if env != "development" && apiKey == "" {
		return fmt.Errorf("API_KEY environment variable is required")
	}

	authProvider := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("x-api-key", apiKey)
		return nil
	}

	client, err := citadel.NewClientWithResponses(
		citadelURL,
		citadel.WithRequestEditorFn(authProvider),
	)
	if err != nil {
		return fmt.Errorf("failed to create citadel client: %w", err)
	}
	getGitHubTokenCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.GetGithubAccessTokenWithResponse(getGitHubTokenCtx)
	if err != nil {
		return fmt.Errorf("failed to get GitHub access token: %w", err)
	}
	if resp.JSON200 == nil {
		return fmt.Errorf("failed to get GitHub access token: %w", err)
	}
	accessToken := resp.JSON200.AccessToken

	auth := &ghttp.BasicAuth{
		Username: "git",
		Password: accessToken,
	}

	cloneOptions := &git.CloneOptions{
		URL:      repoURL,
		Auth:     auth,
		Progress: os.Stdout,
		Depth:    1,
	}

	_, err = git.PlainClone(destPath, false, cloneOptions)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			logger.Warn(ctx, "Repository already exists, skipping clone", "path", destPath)
			return nil
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	logger.Info(ctx, "Repository cloned successfully")
	return nil
}

func GetRepoName(repoURL string) (string, error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid repository URL: %s", repoURL)
	}
	return parts[len(parts)-1], nil
}
