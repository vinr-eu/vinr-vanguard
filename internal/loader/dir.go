package loader

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/internal/defs/v1"
	"vinr.eu/vanguard/internal/logger"
	"vinr.eu/vanguard/internal/serializer"
)

type Store struct {
	Environment *v1.Environment
	Services    map[string]*v1.Service
}

func LoadDir(ctx context.Context, path string) (*Store, error) {
	store := &Store{
		Services: make(map[string]*v1.Service),
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			logger.Warn(ctx, "Failed to read file", "path", fullPath, "error", err)
			continue
		}

		obj, err := serializer.Decode(ctx, data)
		if err != nil {
			logger.Debug(ctx, "Ignoring file", "file", entry.Name(), "reason", err)
			continue
		}

		switch o := obj.(type) {
		case *v1.Service:
			store.Services[o.Name] = o
			logger.Info(ctx, "Loaded resource", "kind", "Service", "name", o.Name, "file", entry.Name())

		case *v1.Environment:
			if store.Environment != nil {
				logger.Warn(ctx, "Multiple Environment objects found in directory, using latest", "file", entry.Name())
			}
			store.Environment = o
			logger.Info(ctx, "Loaded resource", "kind", "Environment", "name", o.Name, "file", entry.Name())

		default:
			logger.Warn(ctx, "Loaded unknown object type", "file", entry.Name())
		}
	}

	if store.Environment != nil {
		for _, imp := range store.Environment.Imports {
			importPath := filepath.Join(path, imp)
			err := loadImport(ctx, store, importPath)
			if err != nil {
				logger.Warn(ctx, "Failed to load import", "path", importPath, "error", err)
			}
		}

		for name, override := range store.Environment.Overrides {
			if svc, ok := store.Services[name]; ok {
				applyOverride(svc, override)
				logger.Info(ctx, "Applied override", "service", name)
			} else {
				logger.Warn(ctx, "Override defined for non-existent service", "service", name)
			}
		}
	}

	return store, nil
}

func loadImport(ctx context.Context, store *Store, path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		fullPath := filepath.Join(path, entry.Name())
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		obj, err := serializer.Decode(ctx, data)
		if err != nil {
			continue
		}

		if svc, ok := obj.(*v1.Service); ok {
			store.Services[svc.Name] = svc
			logger.Info(ctx, "Loaded imported service", "name", svc.Name, "path", fullPath)
		}
	}

	return nil
}

func applyOverride(svc *v1.Service, override v1.ServiceOverride) {
	if override.Branch != nil {
		svc.Branch = override.Branch
	}

	if len(override.Variables) > 0 {
		for _, v := range override.Variables {
			found := false
			for i, existing := range svc.Variables {
				if existing.Name == v.Name {
					svc.Variables[i] = v
					found = true
					break
				}
			}
			if !found {
				svc.Variables = append(svc.Variables, v)
			}
		}
	}
}
