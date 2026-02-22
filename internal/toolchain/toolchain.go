package toolchain

import (
	"context"
	"errors"

	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrUnsupportedEngine = errors.New("toolchain: unsupported engine")
	ErrProvisionFailed   = errors.New("toolchain: provision failed")
)

type Toolchain interface {
	Provision(ctx context.Context, version string) (string, error)
}

func New(engine string, cacheDir string) (Toolchain, error) {
	switch engine {
	case "node":
		return NewNodeToolchain(cacheDir), nil
	case "openjdk":
		return NewOpenJDKToolchain(cacheDir), nil
	default:
		return nil, errs.WrapMsg(ErrUnsupportedEngine, engine)
	}
}
