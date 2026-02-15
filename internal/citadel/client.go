package citadel

import (
	"context"
	"fmt"
	"net/http"
	"time"

	gen "vinr.eu/vanguard/api/citadel/v1"
	"vinr.eu/vanguard/internal/config"
)

type Client struct {
	api *gen.ClientWithResponses
}

func New(ctx context.Context, cfg *config.Config) (*Client, error) {
	apiKeyInjector := func(ctx context.Context, req *http.Request) error {
		req.Header.Set("x-api-key", cfg.CitadelAPIKey)
		return nil
	}

	apiClient, err := gen.NewClientWithResponses(
		cfg.CitadelURL,
		gen.WithRequestEditorFn(apiKeyInjector),
		gen.WithHTTPClient(&http.Client{
			Timeout: 10 * time.Second,
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create citadel client: %w", err)
	}

	client := &Client{
		api: apiClient,
	}

	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Ping(startupCtx); err != nil {
		return nil, fmt.Errorf("citadel startup check failed: %w", err)
	}

	return client, nil
}

func (c *Client) GetGithubAccessToken(ctx context.Context) (string, error) {
	resp, err := c.api.GetGithubAccessTokenWithResponse(ctx)
	err = validateResponse(err, resp, func() *gen.GetGitHubAccessTokenResponse {
		return resp.JSON200
	})
	if err != nil {
		return "", fmt.Errorf("failed to get GitHub access token: %w", err)
	}

	return resp.JSON200.AccessToken, nil
}

func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.api.GetPingWithResponse(ctx)

	return validateResponse(err, resp, func() *gen.PingResponse {
		return resp.JSON200
	})
}

type statusCoder interface {
	StatusCode() int
}

func validateResponse[T any](err error, resp statusCoder, getPayload func() *T) error {
	if err != nil {
		return fmt.Errorf("network error: %w", err)
	}

	if resp == nil {
		return fmt.Errorf("received empty response from citadel")
	}

	code := resp.StatusCode()
	if code < 200 || code >= 300 {
		return fmt.Errorf("api error: http status %d", code)
	}

	payload := getPayload()

	if payload == nil {
		return fmt.Errorf("api error: status was %d but success payload was nil", code)
	}

	return nil
}
