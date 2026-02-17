package defs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/internal/defs/v1"
	"vinr.eu/vanguard/internal/errs"
)

var (
	ErrLoadFailed     = errors.New("defs: load failed")
	ErrReadFailed     = errors.New("defs: read failed")
	ErrDecodeFailed   = errors.New("defs: decode failed")
	ErrNoEnvironment  = errors.New("defs: missing environment")
	ErrDupEnvironment = errors.New("defs: duplicate environment")
	ErrImportFailed   = errors.New("defs: import failed")
)

type Store struct {
	Environment *Environment
	Services    map[string]*Service
}

func NewStore() *Store {
	return &Store{
		Services: make(map[string]*Service),
	}
}

func (s *Store) Load(rootPath string) error {
	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		return s.loadFile(path)
	})
	if err != nil {
		return errs.WrapMsg(ErrLoadFailed, "walk failed at "+rootPath, err)
	}
	if s.Environment == nil {
		return errs.WrapMsg(ErrNoEnvironment, "checked "+rootPath, nil)
	}
	return s.processEnvironment(rootPath)
}

func (s *Store) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return errs.WrapMsg(ErrReadFailed, path, err)
	}
	obj, err := decode(data)
	if err != nil {
		return errs.WrapMsg(ErrDecodeFailed, path, err)
	}
	switch o := obj.(type) {
	case *v1.Service:
		s.Services[o.Name] = mapServiceV1(o)
	case *v1.Environment:
		if s.Environment != nil {
			return errs.WrapMsg(ErrDupEnvironment, path, nil)
		}
		s.Environment = mapEnvironmentV1(o)
	}
	return nil
}

func (s *Store) processEnvironment(rootPath string) error {
	for _, imp := range s.Environment.Imports {
		importPath := filepath.Join(rootPath, filepath.Clean(imp))
		if err := s.loadImport(importPath); err != nil {
			return errs.WrapMsg(ErrImportFailed, imp, err)
		}
	}
	for name, override := range s.Environment.Overrides {
		svc, ok := s.Services[name]
		if !ok {
			slog.Warn("skipping override: service not found", "service", name)
			continue
		}
		s.applyOverride(svc, override)
	}
	return nil
}

func (s *Store) loadImport(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return errs.WrapMsg(ErrImportFailed, path, err)
	}
	return filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".json") {
			return nil
		}
		return s.loadFile(p)
	})
}

func (s *Store) applyOverride(svc *Service, override ServiceOverride) {
	if override.Branch != nil {
		svc.Branch = *override.Branch
	}
	if override.IngressHost != nil {
		svc.IngressHost = override.IngressHost
	}
	for _, v := range override.Variables {
		updateOrAppendVariable(svc, v)
	}
}

func decode(data []byte) (any, error) {
	var meta struct {
		Kind       string `json:"kind"`
		APIVersion string `json:"apiVersion"`
	}

	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, errs.Wrap(ErrDecodeFailed, err)
	}

	switch meta.APIVersion {
	case "v1", "":
		return decodeV1(meta.Kind, data)
	default:
		return errs.WrapMsg(ErrDecodeFailed, "unsupported version "+meta.APIVersion, nil), nil
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
		return nil, errs.WrapMsg(ErrDecodeFailed, "unknown kind "+kind, nil)
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
