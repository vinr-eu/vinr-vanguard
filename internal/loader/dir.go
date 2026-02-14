package loader

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"vinr.eu/vanguard/defs/v1"
	"vinr.eu/vanguard/internal/serializer"
)

// Store holds the lists of all parsed objects found in the directory.
type Store struct {
	Services []v1.Service
	// Add Secrets []v1.Secret here later
}

// LoadDir scans a directory for JSON files and populates a Store.
// It silently ignores files that do not match known Kinds (e.g., package.json).
func LoadDir(path string) (*Store, error) {
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
			slog.Warn("failed to read file", "path", fullPath, "error", err)
			continue
		}

		obj, err := serializer.Decode(data)
		if err != nil {
			// CRITICAL: We use Debug level here.
			// If it's a 'package.json' or a 'tsconfig.json', Decode returns an error.
			// We don't want to spam the logs, so we silence it.
			slog.Debug("ignoring file", "file", entry.Name(), "reason", err)
			continue
		}

		// 4. Type Switch to store it in the correct list
		switch o := obj.(type) {
		case *v1.Service:
			store.Services = append(store.Services, *o)
			slog.Info("loaded resource", "kind", "Service", "name", o.Name, "file", entry.Name())

		// Future proofing:
		// case *v1.Secret:
		//     store.Secrets = append(store.Secrets, *o)

		default:
			// This happens if Decode() works but we forgot to add a case here
			slog.Warn("loaded unknown object type", "file", entry.Name())
		}
	}

	return store, nil
}
