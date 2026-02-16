package deployment

import (
	"context"
	"fmt"
	"strings"

	"vinr.eu/vanguard/internal/defs"
)

type Deployment interface {
	Install(ctx context.Context) error
	Start(ctx context.Context) error
	Stop() error
}

func New(svc *defs.Service, repoPath, binDir string) (Deployment, error) {
	engine := strings.ToLower(svc.Runtime.Engine)
	if engine == "" {
		engine = "node"
	}

	switch engine {
	case "node":
		return NewNodeDeployment(svc, repoPath, binDir), nil
	default:
		return nil, fmt.Errorf("unsupported svc runtime: %s", svc.Runtime)
	}
}
