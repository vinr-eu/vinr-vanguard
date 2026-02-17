package citadel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	gen "vinr.eu/vanguard/api/citadel/v1"
)

var (
	ErrInitFailed    = errors.New("citadel: client init failed")
	ErrNetwork       = errors.New("citadel: network error")
	ErrEmptyResponse = errors.New("citadel: empty response")
	ErrPayloadNil    = errors.New("citadel: payload nil")
	ErrUnauthorized  = errors.New("citadel: unauthorized (401)")
	ErrNotFound      = errors.New("citadel: not found (404)")
	ErrApiFailure    = errors.New("citadel: unexpected api response")
)

type Client struct {
	api     *gen.ClientWithResponses
	apiKey  string
	timeout time.Duration
}

type Option func(*Client)

func WithAPIKey(key string) Option {
	return func(c *Client) {
		c.apiKey = key
	}
}

func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.timeout = d
	}
}

func NewClient(baseURL string, opts ...Option) (*Client, error) {
	c := &Client{
		timeout: 10 * time.Second,
	}
	for _, opt := range opts {
		opt(c)
	}
	httpClient := &http.Client{
		Timeout: c.timeout,
	}
	apiClient, err := gen.NewClientWithResponses(
		baseURL,
		gen.WithHTTPClient(httpClient),
		gen.WithRequestEditorFn(c.authenticate),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInitFailed, err)
	}
	c.api = apiClient
	return c, nil
}

func (c *Client) authenticate(_ context.Context, req *http.Request) error {
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	return nil
}

func (c *Client) String() string {
	return "citadel.Client{redacted}"
}

type statusCoder interface {
	StatusCode() int
}

func (c *Client) GetGithubAccessToken(ctx context.Context) (string, error) {
	resp, err := c.api.GetGithubAccessTokenWithResponse(ctx)
	if err := validateResponse(err, resp, func() bool { return resp.JSON200 != nil }); err != nil {
		return "", err
	}
	return resp.JSON200.AccessToken, nil
}

func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.api.GetPingWithResponse(ctx)
	return validateResponse(err, resp, func() bool { return resp.JSON200 != nil })
}

func validateResponse(err error, resp statusCoder, hasPayload func() bool) error {
	if err != nil {
		return ErrNetwork
	}
	if resp == nil {
		return ErrEmptyResponse
	}
	switch resp.StatusCode() {
	case 200, 201, 204:
		if !hasPayload() {
			return ErrPayloadNil
		}
		return nil
	case 401, 403:
		return ErrUnauthorized
	case 404:
		return ErrNotFound
	default:
		return ErrApiFailure
	}
}
