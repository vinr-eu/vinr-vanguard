package source

import (
	"context"
	"fmt"
	"strings"
)

type TokenProvider func(ctx context.Context) (string, error)

type Source interface {
	Fetch(ctx context.Context, dest string) error
}

func New(repoURL, branch string, githubTokenProvider TokenProvider) (Source, error) {
	switch {
	case strings.Contains(repoURL, "github.com"):
		return NewGitHubSource(repoURL, branch, githubTokenProvider), nil
	default:
		return nil, fmt.Errorf("unsupported source provider: %s", repoURL)
	}
}
