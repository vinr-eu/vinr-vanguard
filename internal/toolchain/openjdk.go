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

type OpenJDKToolchain struct {
	cacheDir string
}

func NewOpenJDKToolchain(cacheDir string) *OpenJDKToolchain {
	return &OpenJDKToolchain{cacheDir: cacheDir}
}

func (t *OpenJDKToolchain) Provision(ctx context.Context, version string) (string, error) {
	if version == "" {
		version = "25"
	}
	installDir := filepath.Join(t.cacheDir, "toolchains", "openjdk", version)
	marker := filepath.Join(installDir, "release")
	if _, err := os.Stat(marker); err == nil {
		return t.findBinDir(installDir)
	}
	slog.Info("provisioning openjdk", "version", version)
	tmpDir := installDir + ".tmp"
	defer os.RemoveAll(tmpDir)
	if err := t.downloadAndExtract(ctx, version, tmpDir); err != nil {
		return "", errs.WrapMsg(ErrProvisionFailed, "openjdk:"+version, err)
	}
	os.RemoveAll(installDir)
	if err := os.Rename(tmpDir, installDir); err != nil {
		return "", errs.Wrap(ErrProvisionFailed, err)
	}
	return t.findBinDir(installDir)
}

func (t *OpenJDKToolchain) findBinDir(root string) (string, error) {
	paths := []string{
		filepath.Join(root, "bin"),
		filepath.Join(root, "Contents", "Home", "bin"),
	}
	entries, _ := os.ReadDir(root)
	for _, e := range entries {
		if e.IsDir() {
			paths = append(paths, filepath.Join(root, e.Name(), "bin"))
			paths = append(paths, filepath.Join(root, e.Name(), "Contents", "Home", "bin"))
		}
	}
	for _, p := range paths {
		if _, err := os.Stat(filepath.Join(p, "java")); err == nil {
			return p, nil
		}
		if _, err := os.Stat(filepath.Join(p, "java.exe")); err == nil {
			return p, nil
		}
	}
	return "", errs.WrapMsg(ErrProvisionFailed, "java binary not found in "+root, nil)
}

func (t *OpenJDKToolchain) downloadAndExtract(ctx context.Context, version, dest string) (err error) {
	req, _ := http.NewRequestWithContext(ctx, "GET", t.getURL(version), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, resp.Body.Close()) }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("adoptium api: %s", resp.Status)
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
		target := filepath.Join(dest, header.Name)
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

func (t *OpenJDKToolchain) writeFile(path string, r io.Reader, mode os.FileMode) (err error) {
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

func (t *OpenJDKToolchain) getURL(v string) string {
	arch := runtime.GOARCH
	if arch == "amd64" {
		arch = "x64"
	}
	if arch == "arm64" {
		arch = "aarch64"
	}
	featureVersion := v
	if parts := strings.Split(v, "."); len(parts) > 1 {
		featureVersion = parts[0]
	}
	return fmt.Sprintf("https://api.adoptium.net/v3/binary/latest/%s/ga/%s/%s/jdk/hotspot/normal/eclipse", featureVersion, runtime.GOOS, arch)
}
