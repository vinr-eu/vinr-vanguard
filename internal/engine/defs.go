package engine

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/internal/defs/v1"
)

type Store struct {
	Environment *v1.Environment
	Services    map[string]*v1.Service
}

func LoadDir(path string) (*Store, error) {
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
			slog.Warn("Failed to read file", "path", fullPath, "error", err)
			continue
		}

		obj, err := decode(data)
		if err != nil {
			slog.Debug("Ignoring file", "file", entry.Name(), "reason", err)
			continue
		}

		switch o := obj.(type) {
		case *v1.Service:
			store.Services[o.Name] = o
			slog.Info("Loaded resource", "kind", "Service", "name", o.Name, "file", entry.Name())

		case *v1.Environment:
			if store.Environment != nil {
				slog.Warn("Multiple Environment objects found in directory, using latest", "file", entry.Name())
			}
			store.Environment = o
			slog.Info("Loaded resource", "kind", "Environment", "name", o.Name, "file", entry.Name())

		default:
			slog.Warn("Loaded unknown object type", "file", entry.Name())
		}
	}

	if store.Environment != nil {
		for _, imp := range store.Environment.Imports {
			importPath := filepath.Join(path, imp)
			err := loadImport(store, importPath)
			if err != nil {
				slog.Warn("Failed to load import", "path", importPath, "error", err)
			}
		}

		for name, override := range store.Environment.Overrides {
			if svc, ok := store.Services[name]; ok {
				applyOverride(svc, override)
				slog.Info("Applied override", "service", name)
			} else {
				slog.Warn("Override defined for non-existent service", "service", name)
			}
		}
	}

	return store, nil
}

func loadImport(store *Store, path string) error {
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

		obj, err := decode(data)
		if err != nil {
			continue
		}

		if svc, ok := obj.(*v1.Service); ok {
			store.Services[svc.Name] = svc
			slog.Info("Loaded imported service", "name", svc.Name, "path", fullPath)
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

func decode(data []byte) (interface{}, error) {
	slog.Debug("Starting decode", "size_bytes", len(data))
	var header v1.TypeMeta
	if err := json.Unmarshal(data, &header); err != nil {
		slog.Error("Failed to parse json header", "error", err)
		return nil, fmt.Errorf("could not parse header: %w", err)
	}

	switch header.Kind {
	case "Service":
		var svc v1.Service
		if err := json.Unmarshal(data, &svc); err != nil {
			slog.Error("Failed to unmarshal service body", "error", err)
			return nil, fmt.Errorf("invalid Service: %w", err)
		}

		slog.Info("Successfully decoded resource", "name", svc.Name)
		return &svc, nil

	case "Environment":
		var env v1.Environment
		if err := json.Unmarshal(data, &env); err != nil {
			slog.Error("Failed to unmarshal environment body", "error", err)
			return nil, fmt.Errorf("invalid Environment: %w", err)
		}

		slog.Info("Successfully decoded resource", "name", env.Name)
		return &env, nil

	default:
		slog.Warn("Unknown resource kind encountered")
		return nil, fmt.Errorf("unknown kind: %s", header.Kind)
	}
}
