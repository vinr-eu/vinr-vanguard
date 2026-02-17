package deployment

import (
	"context"
	"errors"
	"strings"

	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrUnsupportedEngine = errors.New("deployment: unsupported runtime engine")
	ErrInvalidConfig     = errors.New("deployment: invalid service configuration")
)

type Deployment interface {
	Install(ctx context.Context) error
	Start(ctx context.Context) error
	Stop() error
}

func New(svc *defs.Service, repoPath, binDir string) (Deployment, error) {
	if svc == nil {
		return nil, errs.WrapMsg(ErrInvalidConfig, "service definition is nil", nil)
	}
	engine := strings.ToLower(svc.Runtime.Engine)
	if engine == "" {
		engine = "node"
	}
	switch engine {
	case "node":
		return NewNodeDeployment(svc, repoPath, binDir), nil
	case "openjdk":
		return NewOpenJDKDeployment(svc, repoPath, binDir), nil
	default:
		return nil, errs.WrapMsg(ErrUnsupportedEngine, engine, nil)
	}
}
