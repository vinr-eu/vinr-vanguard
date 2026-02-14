package serializer

import (
	"context"
	"encoding/json"
	"fmt"

	"vinr.eu/vanguard/defs/v1"
	"vinr.eu/vanguard/internal/logger"
)

func Decode(ctx context.Context, data []byte) (interface{}, error) {
	logger.Debug(ctx, "Starting decode", "size_bytes", len(data))
	var header v1.TypeMeta
	if err := json.Unmarshal(data, &header); err != nil {
		logger.Error(ctx, "Failed to parse json header", "error", err)
		return nil, fmt.Errorf("could not parse header: %w", err)
	}

	switch header.Kind {
	case "Service":
		var svc v1.Service
		if err := json.Unmarshal(data, &svc); err != nil {
			logger.Error(ctx, "Failed to unmarshal service body", "error", err)
			return nil, fmt.Errorf("invalid Service: %w", err)
		}

		logger.Info(ctx, "Successfully decoded resource", "name", svc.Name)
		return &svc, nil

	default:
		logger.Warn(ctx, "Unknown resource kind encountered")
		return nil, fmt.Errorf("unknown kind: %s", header.Kind)
	}
}
