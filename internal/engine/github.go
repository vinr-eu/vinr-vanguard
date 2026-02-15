package engine

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	ghttp "github.com/go-git/go-git/v5/plumbing/transport/http"
	"vinr.eu/vanguard/internal/citadel"
)

func cloneRepository(ctx context.Context, repoURL, destPath string, client *citadel.Client) error {
	slog.Info("Cloning repository", "url", repoURL, "destination", destPath)

	if client == nil {
		return fmt.Errorf("citadel client is not initialized")
	}
	getGitHubTokenCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	accessToken, err := client.GetGithubAccessToken(getGitHubTokenCtx)
	if err != nil {
		return fmt.Errorf("failed to get GitHub access token: %w", err)
	}

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
			slog.Warn("Repository already exists, skipping clone", "path", destPath)
			return nil
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	slog.Info("Repository cloned successfully")
	return nil
}

func getRepoName(repoURL string) (string, error) {
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("invalid repository URL: %s", repoURL)
	}
	return parts[len(parts)-1], nil
}
