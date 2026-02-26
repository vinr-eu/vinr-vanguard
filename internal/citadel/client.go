package citadel

import (
	"context"
	"errors"
	"net/http"
	"time"

	gen "vinr.eu/vanguard/api/citadel/v1"
	"vinr.eu/vanguard/internal/defs"
	"vinr.eu/vanguard/internal/errs"
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
	nodeID  string
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

func WithNodeID(id string) Option {
	return func(c *Client) {
		c.nodeID = id
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
		return nil, errs.Wrap(ErrInitFailed, err)
	}
	c.api = apiClient
	return c, nil
}

func (c *Client) authenticate(_ context.Context, req *http.Request) error {
	if c.apiKey != "" {
		req.Header.Set("x-api-key", c.apiKey)
	}
	if c.nodeID != "" {
		req.Header.Set("x-node-id", c.nodeID)
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

func (c *Client) GetNodeConfig(ctx context.Context, id string) ([]*defs.Service, error) {
	resp, err := c.api.GetNodeIdGetConfigWithResponse(ctx, id)
	if err := validateResponse(err, resp, func() bool { return resp.JSON200 != nil }); err != nil {
		return nil, err
	}

	services := make([]*defs.Service, len(resp.JSON200.ServiceDeployments))
	for i, s := range resp.JSON200.ServiceDeployments {
		var runScript string
		if s.RunScript != nil {
			runScript = *s.RunScript
		}

		var vars []defs.Variable
		if s.Variables != nil {
			vars = make([]defs.Variable, len(*s.Variables))
			for j, v := range *s.Variables {
				vars[j] = defs.Variable{
					Name:  v.Name,
					Value: v.Value,
					Ref:   v.Ref,
				}
			}
		}

		services[i] = &defs.Service{
			Name: s.Name,
			Runtime: defs.RuntimeSpec{
				Engine:  s.Runtime.Engine,
				Version: s.Runtime.Version,
			},
			GitURL:      s.GitUrl,
			Branch:      s.Branch,
			Port:        s.Port,
			RunScript:   runScript,
			IngressHost: s.IngressHost,
			Variables:   vars,
		}
	}

	return services, nil
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
