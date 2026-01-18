package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client sends webhook requests to the target service.
type Client struct {
	httpClient *http.Client
	baseURL    string
	verbose    bool
}

// Response contains the result of a webhook request.
type Response struct {
	StatusCode int
	Body       []byte
	Duration   time.Duration
	Headers    http.Header
}

// NewClient creates a new webhook client.
func NewClient(baseURL string, verbose bool) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		verbose: verbose,
	}
}

// Send delivers a webhook payload to the target endpoint.
func (c *Client) Send(ctx context.Context, endpoint string, payload []byte, headers http.Header) (*Response, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	for key, values := range headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Duration:   time.Since(start),
		Headers:    resp.Header,
	}, nil
}

// SendRequest is a convenience method that accepts a Request struct.
func (c *Client) SendRequest(ctx context.Context, r *Request) (*Response, error) {
	return c.Send(ctx, r.Endpoint, r.Payload, r.Headers)
}

// Request encapsulates all data needed to send a webhook.
type Request struct {
	Provider string
	Endpoint string
	Payload  []byte
	Headers  http.Header
}

// String returns a human-readable representation of the request.
func (r *Request) String() string {
	return fmt.Sprintf("Provider: %s\nEndpoint: %s\nPayload:\n%s", r.Provider, r.Endpoint, string(r.Payload))
}
