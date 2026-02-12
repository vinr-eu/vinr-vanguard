package code

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

// Checkout clones a repository to a specific destination using an access token.
// It uses a 'shallow clone' (depth=1) for speed since we likely don't need the full history.
func Checkout(repoURL, accessToken, destPath string) error {
	slog.Info("cloning repository", "url", repoURL, "destination", destPath)

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

	// PlainClone downloads the repo into a directory that is NOT a git repo itself
	_, err := git.PlainClone(destPath, false, cloneOptions)
	if err != nil {
		if errors.Is(err, git.ErrRepositoryAlreadyExists) {
			slog.Warn("repository already exists, skipping clone", "path", destPath)
			return nil
		}
		return fmt.Errorf("failed to clone repository: %w", err)
	}

	slog.Info("repository cloned successfully")
	return nil
}
