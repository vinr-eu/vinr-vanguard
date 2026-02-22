package source

import (
	"context"
	"errors"
	"strings"

	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrUnsupportedProvider = errors.New("source: unsupported provider")
)

type TokenProvider func(ctx context.Context) (string, error)

type Source interface {
	Fetch(ctx context.Context, dest string) error
}

func New(repoURL, branch string, tp TokenProvider) (Source, error) {
	switch {
	case strings.Contains(repoURL, "github.com"):
		return NewGitHubSource(repoURL, branch, tp), nil
	default:
		return nil, errs.WrapMsg(ErrUnsupportedProvider, repoURL)
	}
}
