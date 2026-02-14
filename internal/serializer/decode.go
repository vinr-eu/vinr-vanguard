package serializer

import (
	"encoding/json"
	"fmt"
	"log/slog" // Standard library structured logger

	"vinr.eu/vanguard/defs/v1"
)

// Decode takes raw bytes and returns a specific struct (Service, Secret, etc.)
func Decode(data []byte) (interface{}, error) {
	slog.Debug("starting decode", "size_bytes", len(data))
	var header v1.TypeMeta
	if err := json.Unmarshal(data, &header); err != nil {
		slog.Error("failed to parse json header", "error", err)
		return nil, fmt.Errorf("could not parse header: %w", err)
	}

	log := slog.With(
		"kind", header.Kind,
		"apiVersion", header.APIVersion,
	)

	switch header.Kind {
	case "Service":
		var svc v1.Service
		if err := json.Unmarshal(data, &svc); err != nil {
			log.Error("failed to unmarshal service body", "error", err)
			return nil, fmt.Errorf("invalid Service: %w", err)
		}

		log.Info("successfully decoded resource", "name", svc.Name)
		return &svc, nil

	default:
		log.Warn("unknown resource kind encountered")
		return nil, fmt.Errorf("unknown kind: %s", header.Kind)
	}
}
