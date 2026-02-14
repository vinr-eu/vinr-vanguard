package loader

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/defs/v1"
	"vinr.eu/vanguard/internal/logger"
	"vinr.eu/vanguard/internal/serializer"
)

type Store struct {
	Services []v1.Service
}

func LoadDir(ctx context.Context, path string) (*Store, error) {
	store := &Store{
		Services: make([]v1.Service, 0),
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
			store.Services = append(store.Services, *o)
			logger.Info(ctx, "Loaded resource", "kind", "Service", "name", o.Name, "file", entry.Name())

		default:
			logger.Warn(ctx, "Loaded unknown object type", "file", entry.Name())
		}
	}

	return store, nil
}
