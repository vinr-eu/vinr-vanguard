package source

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-github/v69/github"
	"golang.org/x/oauth2"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrAuthFailed   = errors.New("github: auth token retrieval failed")
	ErrRepoInvalid  = errors.New("github: invalid repository URL")
	ErrFetchFailed  = errors.New("github: failed to fetch repository archive")
	ErrUnpackFailed = errors.New("github: failed to unpack repository")
)

type GitHubSource struct {
	repoURL       string
	branch        string
	tokenProvider TokenProvider
}

func NewGitHubSource(repoURL, branch string, tp TokenProvider) *GitHubSource {
	if branch == "" {
		branch = "main"
	}
	return &GitHubSource{
		repoURL:       repoURL,
		branch:        branch,
		tokenProvider: tp,
	}
}

func (s *GitHubSource) Fetch(ctx context.Context, dest string) error {
	token, err := s.tokenProvider(ctx)
	if err != nil {
		return errs.Wrap(ErrAuthFailed, err)
	}

	owner, repo, err := s.parseRepoURL()
	if err != nil {
		return errs.Wrap(ErrRepoInvalid, err)
	}

	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	opt := &github.RepositoryContentGetOptions{Ref: s.branch}
	url, _, err := client.Repositories.GetArchiveLink(ctx, owner, repo, github.Tarball, opt, 3)
	if err != nil {
		return errs.Wrap(ErrFetchFailed, err)
	}

	slog.Info("downloading repository archive", "owner", owner, "repo", repo, "branch", s.branch)
	resp, err := tc.Get(url.String())
	if err != nil {
		return errs.Wrap(ErrFetchFailed, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return errs.Wrap(ErrFetchFailed, fmt.Errorf("unexpected status: %s", resp.Status))
	}
	if err := os.MkdirAll(dest, 0755); err != nil {
		return errs.Wrap(ErrUnpackFailed, err)
	}
	if err := s.unpackTarball(resp.Body, dest); err != nil {
		return errs.Wrap(ErrUnpackFailed, err)
	}
	if entries, err := os.ReadDir(dest); err == nil {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		slog.Info("unpacked repository", "dest", dest, "entries", names)
	}

	return nil
}

func (s *GitHubSource) parseRepoURL() (string, string, error) {
	trimmed := strings.TrimPrefix(s.repoURL, "https://")
	trimmed = strings.TrimSuffix(trimmed, ".git")
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", "", fmt.Errorf("invalid url: %s", s.repoURL)
	}
	for i, p := range parts {
		if p == "github.com" && i+2 < len(parts) {
			return parts[i+1], parts[i+2], nil
		}
	}
	return "", "", fmt.Errorf("could not find owner/repo in: %s", s.repoURL)
}

func (s *GitHubSource) unpackTarball(r io.Reader, dest string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()
	tr := tar.NewReader(gzr)
	var prefix string
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		if header.Typeflag == tar.TypeXGlobalHeader || header.Typeflag == tar.TypeXHeader {
			continue
		}
		if prefix == "" {
			prefix = header.Name
			if !strings.HasSuffix(prefix, "/") {
				prefix += "/"
			}
			slog.Info("detected tarball prefix", "prefix", prefix)
			continue
		}

		// Strip the prefix (root directory)
		name := strings.TrimPrefix(header.Name, prefix)
		if name == "" {
			continue
		}

		target := filepath.Join(dest, name)
		slog.Info("unpacking file", "header", header.Name, "target", target)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			f, err := os.OpenFile(target, os.O_CREATE|os.O_RDWR|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			if err := os.Symlink(header.Linkname, target); err != nil {
				return err
			}
		}
	}
	return nil
}
