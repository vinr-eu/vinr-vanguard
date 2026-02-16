package toolchain

import (
	"context"
	"fmt"
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
		return nil, fmt.Errorf("no toolchain available for engine: %s", engine)
	}
}
