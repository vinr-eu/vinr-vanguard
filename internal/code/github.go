package code

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"vinr.eu/vanguard/internal/logger"
)

func Checkout(ctx context.Context, repoURL, accessToken, destPath string) error {
	logger.Info(ctx, "cloning repository", "url", repoURL, "destination", destPath)

	auth := &http.BasicAuth{
		Username: "git",
		Password: accessToken,
	}

	cloneOptions := &git.CloneOptions{
		URL:      repoURL,
		Auth:     auth,
		Progress: os.Stdout,
		Depth:    1,
	}

	_, err := git.PlainClone(destPath, false, cloneOptions)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			logger.Warn(ctx, "repository already exists, skipping clone", "path", destPath)
			return nil
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	logger.Info(ctx, "repository cloned successfully")
	return nil
}
