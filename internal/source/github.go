package source

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type GitHubSource struct {
	RepoURL string
	Token   string
}

func (s *GitHubSource) Fetch(ctx context.Context, dest string) error {
	if _, err := os.Stat(dest); err == nil {
		if _, err := git.PlainOpen(dest); err == nil {
			slog.Debug("repository already exists", "path", dest)
			return nil
		}
		return fmt.Errorf("destination exists but is not a git repository: %s", dest)
	}

	auth := &http.BasicAuth{
		Username: "x-access-token",
		Password: s.Token,
	}

	slog.Info("fetching fuel from GitHub", "url", s.RepoURL)

	_, err := git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
		URL:      s.RepoURL,
		Auth:     auth,
		Depth:    1,
		Progress: io.Discard,
	})

	if err != nil {
		return fmt.Errorf("failed to fetch from GitHub: %w", err)
	}

	return nil
}

func (s *GitHubSource) String() string {
	return fmt.Sprintf("GitHubSource{url: %s}", s.RepoURL)
}
