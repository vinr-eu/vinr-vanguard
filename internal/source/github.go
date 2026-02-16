package source

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
)

type GitHubSource struct {
	repoURL          string
	branch           string
	fetchGitHubToken TokenProvider
}

func NewGitHubSource(repoURL, branch string, githubTokenProvider TokenProvider) *GitHubSource {
	if branch == "" {
		branch = "main" // Default safety
	}
	return &GitHubSource{
		repoURL:          repoURL,
		branch:           branch,
		fetchGitHubToken: githubTokenProvider,
	}
}

func (s *GitHubSource) Fetch(ctx context.Context, dest string) error {
	token, err := s.fetchGitHubToken(ctx)
	if err != nil {
		return fmt.Errorf("failed to retrieve auth token: %w", err)
	}

	auth := &http.BasicAuth{
		Username: "x-access-token",
		Password: token,
	}

	branchRef := plumbing.NewBranchReferenceName(s.branch)

	if _, err := os.Stat(dest); err == nil {
		slog.Info("repository exists, syncing...", "path", dest)

		repo, err := git.PlainOpen(dest)
		if err != nil {
			return fmt.Errorf("destination exists but is not a git repository: %w", err)
		}

		w, err := repo.Worktree()
		if err != nil {
			return err
		}

		slog.Debug("cleaning local working directory")
		if err := w.Clean(&git.CleanOptions{Dir: true}); err != nil {
			return fmt.Errorf("failed to clean workspace: %w", err)
		}

		slog.Info("fetching updates from remote", "branch", s.branch)
		err = repo.FetchContext(ctx, &git.FetchOptions{
			Auth:     auth,
			Progress: io.Discard,
			RefSpecs: []config.RefSpec{"+refs/heads/*:refs/remotes/origin/*"}, // Fetch all refs
			Force:    true,
		})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return fmt.Errorf("fetch failed: %w", err)
		}

		if err := w.Checkout(&git.CheckoutOptions{
			Branch: branchRef,
			Force:  true,
		}); err != nil {
			return fmt.Errorf("checkout failed: %w", err)
		}

		remoteRef := plumbing.NewRemoteReferenceName("origin", s.branch)
		remoteHash, err := repo.ResolveRevision(plumbing.Revision(remoteRef))
		if err != nil {
			return fmt.Errorf("failed to resolve remote revision %s: %w", remoteRef, err)
		}

		slog.Info("resetting hard to remote", "hash", remoteHash.String())
		if err := w.Reset(&git.ResetOptions{
			Mode:   git.HardReset,
			Commit: *remoteHash,
		}); err != nil {
			return fmt.Errorf("hard reset failed: %w", err)
		}

		return nil
	}

	slog.Info("cloning new repository", "url", s.repoURL, "branch", s.branch)

	_, err = git.PlainCloneContext(ctx, dest, false, &git.CloneOptions{
		URL:           s.repoURL,
		Auth:          auth,
		ReferenceName: branchRef,
		SingleBranch:  true,
		Depth:         1,
		Progress:      io.Discard,
	})

	if err != nil {
		return fmt.Errorf("failed to clone from GitHub: %w", err)
	}

	return nil
}
