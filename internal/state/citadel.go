package state

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	citadel "vinr.eu/vanguard/api/citadel/v1"
)

// GlobalCitadel Global instance (Singleton)
var GlobalCitadel *CitadelManager

// CitadelState holds the current health of the connection
type CitadelState struct {
	IsAlive   bool
	LastCheck time.Time
	Message   string
}

// CitadelManager handles the connection and state
type CitadelManager struct {
	client *citadel.ClientWithResponses
	state  CitadelState
	mu     sync.RWMutex
}

// InitCitadel Init initializes the global manager and starts the background pinger immediately.
func InitCitadel(baseURL string, interval time.Duration) error {
	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		return fmt.Errorf("API_KEY environment variable is required")
	}

	authProvider := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("x-api-key", apiKey)
		return nil
	}

	client, err := citadel.NewClientWithResponses(
		baseURL,
		citadel.WithRequestEditorFn(authProvider),
	)
	if err != nil {
		return fmt.Errorf("failed to create citadel client: %w", err)
	}

	// Create the manager
	mgr := &CitadelManager{
		client: client,
		state: CitadelState{
			IsAlive: false,
			Message: "Initializing...",
		},
	}

	// Assign to Global Variable
	GlobalCitadel = mgr

	// Start the background pinger immediately (Fire and Forget)
	go mgr.startPingerLoop(context.Background(), interval)

	return nil
}

// GetCitadel Get Helper to get state easily
func GetCitadel() CitadelState {
	if GlobalCitadel == nil {
		return CitadelState{Message: "Not Initialized", IsAlive: false}
	}
	return GlobalCitadel.GetState()
}

// Internal loop logic
func (m *CitadelManager) startPingerLoop(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run the first ping immediately
	m.ping(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.ping(ctx)
		}
	}
}

// ping performs the actual request and updates the state
func (m *CitadelManager) ping(ctx context.Context) {
	// Use a short timeout so the pinger doesn't hang
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := m.client.PingWithResponse(pingCtx)

	m.mu.Lock()
	defer m.mu.Unlock()

	m.state.LastCheck = time.Now()

	if err != nil {
		m.state.IsAlive = false
		m.state.Message = fmt.Sprintf("Network Error: %v", err)
		// Optional: reduce log spam by only logging on state change
		log.Printf("⚠️ Citadel Unreachable: %v", err)
		return
	}

	if resp.JSON200 != nil {
		m.state.IsAlive = true
		status := resp.JSON200.Status()
		m.state.Message = status
	} else {
		m.state.IsAlive = false
		m.state.Message = fmt.Sprintf("HTTP %d", resp.StatusCode())
		log.Printf("⚠️ Citadel returned error: %d", resp.StatusCode())
	}
}

// GetState is safe to call from anywhere
func (m *CitadelManager) GetState() CitadelState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}
