package toolchain

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
	"runtime"
	"strings"

	"vinr.eu/vanguard/internal/errs"
)

type NodeToolchain struct {
	cacheDir string
}

func NewNodeToolchain(cacheDir string) *NodeToolchain {
	return &NodeToolchain{cacheDir: cacheDir}
}

func (t *NodeToolchain) Provision(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = "20.11.0"
	}
	installDir := filepath.Join(t.cacheDir, "toolchains", "node", version)
	binDir := filepath.Join(installDir, "bin")
	exe := filepath.Join(binDir, "node")
	if runtime.GOOS == "windows" {
		binDir = installDir
		exe += ".exe"
	}
	if _, err := os.Stat(exe); err == nil {
		return binDir, nil
	}
	slog.Info("provisioning node", "version", version)
	tmpDir := installDir + ".tmp"
	defer os.RemoveAll(tmpDir)
	if err := t.downloadAndExtract(ctx, version, tmpDir); err != nil {
		return "", errs.WrapMsg(ErrProvisionFailed, "node:"+version, err)
	}
	os.RemoveAll(installDir)
	if err := os.Rename(tmpDir, installDir); err != nil {
		return "", errs.Wrap(ErrProvisionFailed, err)
	}
	return binDir, nil
}

func (t *NodeToolchain) downloadAndExtract(ctx context.Context, version, dest string) (err error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", t.getURL(version), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("http %s", resp.Status)
	}
	gzr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, gzr.Close()) }()
	tr := tar.NewReader(gzr)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		parts := strings.SplitN(header.Name, "/", 2)
		if len(parts) < 2 || parts[1] == "" {
			continue
		}
		target := filepath.Join(dest, parts[1])
		if !strings.HasPrefix(target, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal path: %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := t.writeFile(target, tr, header.FileInfo().Mode()); err != nil {
				return err
			}
		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			os.Remove(target)
			if err := os.Symlink(header.Linkname, target); err != nil {
				continue
			}
		}
	}
	return nil
}

func (t *NodeToolchain) writeFile(path string, r io.Reader, mode os.FileMode) (err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	_, err = io.Copy(f, r)
	return
}

func (t *NodeToolchain) getURL(v string) string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	return fmt.Sprintf("https://nodejs.org/dist/v%s/node-v%s-%s-%s.tar.gz", v, v, runtime.GOOS, arch)
}
