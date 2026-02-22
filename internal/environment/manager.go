package environment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"vinr.eu/vanguard/internal/aws"
	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/deployment"
	"vinr.eu/vanguard/internal/errs"
	"vinr.eu/vanguard/internal/source"
	"vinr.eu/vanguard/internal/toolchain"
)

var (
	ErrBootFailed      = errors.New("environment: boot failed")
	ErrNoSource        = errors.New("environment: no source for definitions")
	ErrProvisionFailed = errors.New("environment: provisioning failed")
	ErrDeployFailed    = errors.New("environment: service deployment failed")
)

type Manager struct {
	workspaceDir         string
	defsStore            *defs.Store
	activeDeployments    map[string]deployment.Deployment
	tokenProvider        source.TokenProvider
	secretsManagerClient *aws.SecretsManagerClient
}

func NewManager(workspaceDir string, tp source.TokenProvider, smc *aws.SecretsManagerClient) *Manager {
	return &Manager{
		workspaceDir:         workspaceDir,
		defsStore:            defs.NewStore().WithSecretsManager(smc),
		activeDeployments:    make(map[string]deployment.Deployment),
		tokenProvider:        tp,
		secretsManagerClient: smc,
	}
}

func (m *Manager) Boot(ctx context.Context, envDefsGitURL string, envDefsDir string) error {
	var envPath string
	if envDefsGitURL != "" && envDefsDir != "" {
		definitionsDir := filepath.Join(m.workspaceDir, "definitions")
		envPath = filepath.Join(definitionsDir, envDefsDir)
		envSrc, err := source.New(envDefsGitURL, "main", m.tokenProvider)
		if err != nil {
			return errs.WrapMsgErr(ErrBootFailed, "source init", err)
		}
		if err := envSrc.Fetch(ctx, definitionsDir); err != nil {
			return errs.WrapMsgErr(ErrBootFailed, "fetch specs", err)
		}
	} else if envDefsDir != "" {
		slog.InfoContext(ctx, "using local env specs", "path", envDefsDir)
		envPath = envDefsDir
	} else {
		return ErrNoSource
	}
	if err := m.defsStore.Load(ctx, envPath); err != nil {
		return errs.WrapMsgErr(ErrBootFailed, "store load: "+envPath, err)
	}
	runtimePaths, err := m.ProvisionAll(ctx)
	if err != nil {
		return errs.Wrap(ErrProvisionFailed, err)
	}
	for _, svc := range m.defsStore.Services {
		key := fmt.Sprintf("%s:%s", svc.Runtime.Engine, svc.Runtime.Version)
		binDir := runtimePaths[key]
		if err := m.deployService(ctx, svc, binDir); err != nil {
			slog.ErrorContext(ctx, "deployment failed", "service", svc.Name, "error", err)
			continue
		}
	}
	return nil
}

func (m *Manager) ProvisionAll(ctx context.Context) (map[string]string, error) {
	required := make(map[string]defs.RuntimeSpec)
	for _, svc := range m.defsStore.Services {
		key := fmt.Sprintf("%s:%s", svc.Runtime.Engine, svc.Runtime.Version)
		required[key] = svc.Runtime
	}
	slog.InfoContext(ctx, "resolving runtimes", "count", len(required))
	results := make(map[string]string)
	for key, spec := range required {
		tc, err := toolchain.New(spec.Engine, m.workspaceDir)
		if err != nil {
			return nil, errs.WrapMsgErr(ErrProvisionFailed, key, err)
		}
		slog.InfoContext(ctx, "provisioning toolchain", "spec", key)
		binDir, err := tc.Provision(ctx, spec.Version)
		if err != nil {
			return nil, errs.WrapMsgErr(ErrProvisionFailed, key, err)
		}
		results[key] = binDir
	}
	return results, nil
}

func (m *Manager) GetServices() map[string]*defs.Service {
	return m.defsStore.Services
}

func (m *Manager) Shutdown() {
	for name, dep := range m.activeDeployments {
		slog.Info("stopping service", "service", name)
		if err := dep.Stop(); err != nil {
			slog.Error("shutdown error", "service", name, "error", err)
		}
	}
}

func (m *Manager) deployService(ctx context.Context, svc *defs.Service, binDir string) error {
	if svc.GitURL == "" {
		return errs.WrapMsg(ErrDeployFailed, "no git url: "+svc.Name)
	}
	repoPath := filepath.Join(m.workspaceDir, "services", svc.Name)
	src, err := source.New(svc.GitURL, svc.Branch, m.tokenProvider)
	if err != nil {
		return errs.WrapMsgErr(ErrDeployFailed, "source init: "+svc.Name, err)
	}
	if err := src.Fetch(ctx, repoPath); err != nil {
		return errs.WrapMsgErr(ErrDeployFailed, "fetch: "+svc.Name, err)
	}
	dep, err := deployment.New(svc, repoPath, binDir)
	if err != nil {
		return errs.WrapMsgErr(ErrDeployFailed, "dep init: "+svc.Name, err)
	}
	if err := dep.Install(ctx); err != nil {
		return errs.WrapMsgErr(ErrDeployFailed, "install: "+svc.Name, err)
	}
	if err := dep.Start(ctx); err != nil {
		return errs.WrapMsgErr(ErrDeployFailed, "start: "+svc.Name, err)
	}
	m.activeDeployments[svc.Name] = dep
	return nil
}
