package state

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	citadel "vinr.eu/vanguard/api/citadel/v1"
	"vinr.eu/vanguard/internal/logger"
)

var GlobalCitadel *CitadelManager

type CitadelState struct {
	IsAlive   bool
	LastCheck time.Time
	Message   string
}

type CitadelManager struct {
	client *citadel.ClientWithResponses
	state  CitadelState
	mu     sync.RWMutex
}

func InitCitadel(interval time.Duration) error {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}
	citadelURL := os.Getenv("CITADEL_URL")
	if env == "development" && citadelURL == "" {
		citadelURL = "http://localhost:9080"
	}
	apiKey := os.Getenv("API_KEY")
	if env != "development" && apiKey == "" {
		return fmt.Errorf("API_KEY environment variable is required")
	}

	authProvider := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("x-api-key", apiKey)
		return nil
	}

	client, err := citadel.NewClientWithResponses(
		citadelURL,
		citadel.WithRequestEditorFn(authProvider),
	)
	if err != nil {
		return fmt.Errorf("failed to create citadel client: %w", err)
	}

	mgr := &CitadelManager{
		client: client,
		state: CitadelState{
			IsAlive: false,
			Message: "Initializing...",
		},
	}

	GlobalCitadel = mgr

	mgr.ping(context.Background())

	go mgr.startPingerLoop(context.Background(), interval)

	return nil
}

func GetCitadel() CitadelState {
	if GlobalCitadel == nil {
		return CitadelState{Message: "Not Initialized", IsAlive: false}
	}
	return GlobalCitadel.GetState()
}

func (m *CitadelManager) startPingerLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.ping(ctx)
		}
	}
}

func (m *CitadelManager) ping(ctx context.Context) {
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.GetPingWithResponse(pingCtx)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.LastCheck = time.Now()

	if err != nil {
		m.state.IsAlive = false
		m.state.Message = fmt.Sprintf("Network Error: %v", err)
		logger.Error(ctx, "Citadel Unreachable", "error", err)
		return
	}

	if resp.JSON200 != nil {
		m.state.IsAlive = true
		status := resp.JSON200.Status
		m.state.Message = status
	} else {
		m.state.IsAlive = false
		m.state.Message = fmt.Sprintf("HTTP %d", resp.StatusCode())
		logger.Error(ctx, "Citadel returned error", "statusCode", resp.StatusCode())
	}
}

func (m *CitadelManager) GetState() CitadelState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}
