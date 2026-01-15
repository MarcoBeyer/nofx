// Package finnhub provides access to the Finnhub API for stock data
// including screening, quotes, and news.
package finnhub

import (
	"fmt"
	"io"
	"net/http"
	"nofx/config"
	"sync"
	"time"
)

const (
	DefaultBaseURL = "https://finnhub.io/api/v1"
	DefaultTimeout = 30 * time.Second
)

// Client is the Finnhub API client
type Client struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
	client  *http.Client
	mu      sync.RWMutex
}

var (
	defaultClient *Client
	clientOnce    sync.Once
)

// DefaultClient returns the singleton default client
func DefaultClient() *Client {
	clientOnce.Do(func() {
		defaultClient = &Client{
			BaseURL: DefaultBaseURL,
			APIKey:  config.Get().FinnhubAPIKey,
			Timeout: DefaultTimeout,
			client:  &http.Client{Timeout: DefaultTimeout},
		}
	})
	return defaultClient
}

// NewClient creates a new Finnhub API client
func NewClient(apiKey string) *Client {
	if apiKey == "" {
		apiKey = config.Get().FinnhubAPIKey
	}
	return &Client{
		BaseURL: DefaultBaseURL,
		APIKey:  apiKey,
		Timeout: DefaultTimeout,
		client:  &http.Client{Timeout: DefaultTimeout},
	}
}

// SetAPIKey updates the API key
func (c *Client) SetAPIKey(apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.APIKey = apiKey
}

// GetAPIKey returns the current API key
func (c *Client) GetAPIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.APIKey
}

// doRequest performs an HTTP GET request with authentication
func (c *Client) doRequest(endpoint string) ([]byte, error) {
	c.mu.RLock()
	baseURL := c.BaseURL
	apiKey := c.APIKey
	c.mu.RUnlock()

	if apiKey == "" {
		return nil, fmt.Errorf("finnhub API key not configured")
	}

	url := baseURL + endpoint
	if url[len(url)-1] != '?' {
		if len(endpoint) > 0 && endpoint[len(endpoint)-1] != '?' {
			if contains(url, "?") {
				url += "&token=" + apiKey
			} else {
				url += "?token=" + apiKey
			}
		}
	} else {
		url += "token=" + apiKey
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return body, &APIError{
			StatusCode: resp.StatusCode,
			Message:    string(body),
		}
	}

	return body, nil
}

// APIError represents an API error response
type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("finnhub API error (status %d): %s", e.StatusCode, e.Message)
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
