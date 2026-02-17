package source

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport/http"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrAuthFailed  = errors.New("github: auth token retrieval failed")
	ErrRepoInvalid = errors.New("github: invalid git repository")
	ErrSyncFailed  = errors.New("github: sync operation failed")
	ErrCloneFailed = errors.New("github: clone operation failed")
	ErrResetFailed = errors.New("github: reset failed")
)

type GitHubSource struct {
	repoURL          string
	branch           string
	fetchGitHubToken TokenProvider
}

func NewGitHubSource(repoURL, branch string, githubTokenProvider TokenProvider) *GitHubSource {
	if branch == "" {
		branch = "main"
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
		return errs.Wrap(ErrAuthFailed, err)
	}
	auth := &http.BasicAuth{Username: "x-access-token", Password: token}
	branchRef := plumbing.NewBranchReferenceName(s.branch)
	if _, err := os.Stat(dest); err == nil {
		slog.Info("repository exists, syncing...", "path", dest)
		repo, err := git.PlainOpen(dest)
		if err != nil {
			return errs.Wrap(ErrRepoInvalid, err)
		}
		w, err := repo.Worktree()
		if err != nil {
			return errs.Wrap(ErrSyncFailed, err)
		}
		if err := w.Clean(&git.CleanOptions{Dir: true}); err != nil {
			return errs.WrapMsg(ErrSyncFailed, "clean failed", err)
		}
		err = repo.FetchContext(ctx, &git.FetchOptions{
			Auth:     auth,
			Progress: io.Discard,
			RefSpecs: []config.RefSpec{"+refs/heads/*:refs/remotes/origin/*"},
			Force:    true,
		})
		if err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return errs.Wrap(ErrSyncFailed, err)
		}
		if err := w.Checkout(&git.CheckoutOptions{Branch: branchRef, Force: true}); err != nil {
			return errs.WrapMsg(ErrSyncFailed, "checkout failed", err)
		}
		remoteRef := plumbing.NewRemoteReferenceName("origin", s.branch)
		remoteHash, err := repo.ResolveRevision(plumbing.Revision(remoteRef))
		if err != nil {
			return errs.WrapMsg(ErrSyncFailed, "resolve remote failed", err)
		}
		if err := w.Reset(&git.ResetOptions{Mode: git.HardReset, Commit: *remoteHash}); err != nil {
			return errs.Wrap(ErrResetFailed, err)
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
		return errs.Wrap(ErrCloneFailed, err)
	}
	return nil
}
