package defs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"vinr.eu/vanguard/internal/aws"
	"vinr.eu/vanguard/internal/defs/v1"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrLoadFailed            = errors.New("defs: load failed")
	ErrReadFailed            = errors.New("defs: read failed")
	ErrDecodeFailed          = errors.New("defs: decode failed")
	ErrNoEnvironment         = errors.New("defs: missing environment")
	ErrDupEnvironment        = errors.New("defs: duplicate environment")
	ErrImportFailed          = errors.New("defs: import failed")
	ErrResolveVariableFailed = errors.New("defs: resolve variable failed")
)

const AwsSecretPrefix = "aws/secrets/"

type Store struct {
	Environment          *Environment
	Services             map[string]*Service
	fetchSecret          func(ctx context.Context, secretID string) (string, error)
	secretsManagerClient *aws.SecretsManagerClient
}

func NewStore() *Store {
	s := &Store{
		Services: make(map[string]*Service),
	}
	s.fetchSecret = s.fetchAwsSecret
	return s
}

func (s *Store) WithSecretsManager(client *aws.SecretsManagerClient) *Store {
	s.secretsManagerClient = client
	return s
}

func (s *Store) Load(ctx context.Context, rootPath string) error {
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(d.Name(), ".json") || strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml")) {
			return nil
		}
		return s.loadFile(path)
	})
	if err != nil {
		return errs.WrapMsgErr(ErrLoadFailed, "walk failed at "+rootPath, err)
	}
	if s.Environment == nil {
		return errs.WrapMsg(ErrNoEnvironment, "checked "+rootPath)
	}
	return s.processEnvironment(ctx, rootPath)
}

func (s *Store) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errs.WrapMsgErr(ErrReadFailed, path, err)
	}
	obj, err := decode(data)
	if err != nil {
		return errs.WrapMsgErr(ErrDecodeFailed, path, err)
	}
	switch o := obj.(type) {
	case *v1.Service:
		s.Services[o.Name] = mapServiceV1(o)
	case *v1.Environment:
		if s.Environment != nil {
			return errs.WrapMsg(ErrDupEnvironment, path)
		}
		s.Environment = mapEnvironmentV1(o)
	}
	return nil
}

func (s *Store) processEnvironment(ctx context.Context, rootPath string) error {
	for _, imp := range s.Environment.Imports {
		importPath := filepath.Join(rootPath, filepath.Clean(imp))
		if err := s.loadImport(importPath); err != nil {
			return errs.WrapMsgErr(ErrImportFailed, imp, err)
		}
	}
	nextPort := 3000
	for _, svc := range s.Services {
		svc.Port = nextPort
		nextPort++
		if err := s.resolveServiceSecrets(ctx, svc); err != nil {
			return err
		}
	}
	for name, override := range s.Environment.Overrides {
		svc, ok := s.Services[name]
		if !ok {
			slog.Warn("skipping override: service not found", "service", name)
			continue
		}
		if err := s.applyOverride(ctx, svc, override); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) loadImport(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errs.WrapMsgErr(ErrImportFailed, path, err)
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !(strings.HasSuffix(d.Name(), ".json") || strings.HasSuffix(d.Name(), ".yaml") || strings.HasSuffix(d.Name(), ".yml")) {
			return nil
		}
		return s.loadFile(p)
	})
}

func (s *Store) applyOverride(ctx context.Context, svc *Service, override ServiceOverride) error {
	if override.Branch != nil {
		svc.Branch = *override.Branch
	}
	if override.Port != nil {
		svc.Port = *override.Port
	}
	if override.IngressHost != nil {
		svc.IngressHost = override.IngressHost
	}
	for _, v := range override.Variables {
		expandedVars, err := s.resolveVariable(ctx, v)
		if err != nil {
			return err
		}
		for _, ev := range expandedVars {
			updateOrAppendVariable(svc, ev)
		}
	}
	return nil
}

func (s *Store) resolveServiceSecrets(ctx context.Context, svc *Service) error {
	var finalVars []Variable
	for _, v := range svc.Variables {
		expanded, err := s.resolveVariable(ctx, v)
		if err != nil {
			return err
		}
		finalVars = append(finalVars, expanded...)
	}
	svc.Variables = finalVars
	return nil
}

func (s *Store) resolveVariable(ctx context.Context, v Variable) ([]Variable, error) {
	if v.Value != nil || v.Ref == nil {
		return []Variable{v}, nil
	}
	if !strings.HasPrefix(*v.Ref, AwsSecretPrefix) {
		return []Variable{v}, nil
	}
	secretID := strings.TrimPrefix(*v.Ref, AwsSecretPrefix)
	secretValue, err := s.fetchSecret(ctx, secretID)
	if err != nil {
		return nil, errs.WrapMsgErr(ErrResolveVariableFailed, v.Name, err)
	}
	var entries map[string]any
	if err := json.Unmarshal([]byte(secretValue), &entries); err == nil {
		var expanded []Variable
		prefix := strings.ToUpper(v.Name)
		for key, value := range entries {
			suffix := strings.ToUpper(key)
			newName := fmt.Sprintf("%s_%s", prefix, suffix)
			val := fmt.Sprintf("%v", value)
			expanded = append(expanded, Variable{
				Name:  newName,
				Value: &val,
			})
		}
		return expanded, nil
	}
	return []Variable{{
		Name:  v.Name,
		Value: &secretValue,
	}}, nil
}

func (s *Store) fetchAwsSecret(ctx context.Context, secretID string) (string, error) {
	if s.secretsManagerClient == nil {
		return "", fmt.Errorf("aws secrets manager client not initialized")
	}
	return s.secretsManagerClient.GetSecret(ctx, secretID)
}

func decode(data []byte) (any, error) {
	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		return nil, errs.Wrap(ErrDecodeFailed, err)
	}
	var meta struct {
		Kind       string `json:"kind"`
		DefVersion string `json:"defVersion"`
	}
	if err := json.Unmarshal(jsonData, &meta); err != nil {
		return nil, errs.Wrap(ErrDecodeFailed, err)
	}
	switch meta.DefVersion {
	case "v1", "":
		return decodeV1(meta.Kind, jsonData)
	default:
		return errs.WrapMsg(ErrDecodeFailed, "unsupported version "+meta.DefVersion), nil
	}
}

func decodeV1(kind string, data []byte) (any, error) {
	switch kind {
	case "Service":
		var svc v1.Service
		if err := json.Unmarshal(data, &svc); err != nil {
			return nil, errs.Wrap(ErrDecodeFailed, err)
		}
		return &svc, nil
	case "Environment":
		var env v1.Environment
		if err := json.Unmarshal(data, &env); err != nil {
			return nil, errs.Wrap(ErrDecodeFailed, err)
		}
		return &env, nil
	default:
		return nil, errs.WrapMsg(ErrDecodeFailed, "unknown kind "+kind)
	}
}

func updateOrAppendVariable(svc *Service, newVal Variable) {
	for i, existing := range svc.Variables {
		if existing.Name == newVal.Name {
			svc.Variables[i] = newVal
			return
		}
	}
	svc.Variables = append(svc.Variables, newVal)
}
