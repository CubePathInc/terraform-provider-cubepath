package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"time"

	"golang.org/x/time/rate"
)

// Client is the HTTP client for the CubePath API
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiToken   string

	// Retry configuration
	maxRetries   int
	retryWaitMin time.Duration
	retryWaitMax time.Duration

	// Rate limiting
	rateLimiter *rate.Limiter

	// Services
	SSHKeys            *SSHKeys
	Projects           *Projects
	VPS                *VPSService
	Baremetal          *BaremetalService
	Networks           *Networks
	Pricing            *Pricing
	FloatingIPs        *FloatingIPs
	Firewall           *FirewallService
	DNS                *DNSService
	LoadBalancer       *LoadBalancerService
	CDN                *CDNService
	Kubernetes         *KubernetesService
	AvailabilityGroups *AvailabilityGroups
}

// ClientOption is a function that configures a Client
type ClientOption func(*Client)

// WithMaxRetries sets the maximum number of retries
func WithMaxRetries(max int) ClientOption {
	return func(c *Client) {
		c.maxRetries = max
	}
}

// WithRetryWaitMin sets the minimum wait time between retries
func WithRetryWaitMin(d time.Duration) ClientOption {
	return func(c *Client) {
		c.retryWaitMin = d
	}
}

// WithRetryWaitMax sets the maximum wait time between retries
func WithRetryWaitMax(d time.Duration) ClientOption {
	return func(c *Client) {
		c.retryWaitMax = d
	}
}

// WithHTTPClient sets a custom HTTP client
func WithHTTPClient(httpClient *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// NewClient creates a new CubePath API client
func NewClient(apiToken, baseURL string, opts ...ClientOption) (*Client, error) {
	if apiToken == "" {
		return nil, fmt.Errorf("api token is required")
	}

	if baseURL == "" {
		baseURL = "https://api.cubepath.com"
	}

	// Default HTTP client with sensible timeouts
	defaultHTTPClient := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
		},
	}

	client := &Client{
		httpClient:   defaultHTTPClient,
		baseURL:      baseURL,
		apiToken:     apiToken,
		maxRetries:   3,
		retryWaitMin: 1 * time.Second,
		retryWaitMax: 30 * time.Second,
		rateLimiter:  rate.NewLimiter(rate.Every(100*time.Millisecond), 10), // 10 req/sec
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	// Initialize services
	client.SSHKeys = NewSSHKeys(client)
	client.Projects = NewProjects(client)
	client.VPS = NewVPSService(client)
	client.Baremetal = NewBaremetalService(client)
	client.Networks = NewNetworks(client)
	client.Pricing = NewPricing(client)
	client.FloatingIPs = NewFloatingIPs(client)
	client.Firewall = NewFirewallService(client)
	client.DNS = NewDNSService(client)
	client.LoadBalancer = NewLoadBalancerService(client)
	client.CDN = NewCDNService(client)
	client.Kubernetes = NewKubernetesService(client)
	client.AvailabilityGroups = NewAvailabilityGroups(client)

	return client, nil
}

// doRequest performs an HTTP request with retry logic
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	var lastErr error

	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		// Exponential backoff with jitter (except on first attempt)
		if attempt > 0 {
			waitTime := c.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(waitTime):
			}
		}

		// Rate limiting
		if err := c.rateLimiter.Wait(ctx); err != nil {
			return nil, err
		}

		// Create the request
		req, err := c.newRequest(ctx, method, path, body)
		if err != nil {
			return nil, err
		}

		// Execute the request
		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Check if we should retry
		if c.shouldRetry(resp) {
			resp.Body.Close()
			lastErr = fmt.Errorf("received status %d", resp.StatusCode)
			continue
		}

		// Success or non-retryable error
		return resp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// newRequest creates a new HTTP request
func (c *Client) newRequest(ctx context.Context, method, path string, body interface{}) (*http.Request, error) {
	url := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Authorization", "Bearer "+c.apiToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	return req, nil
}

// shouldRetry determines if a request should be retried based on the response
func (c *Client) shouldRetry(resp *http.Response) bool {
	// Retry on rate limit
	if resp.StatusCode == http.StatusTooManyRequests {
		return true
	}

	// Retry on server errors (5xx)
	if resp.StatusCode >= 500 {
		return true
	}

	return false
}

// calculateBackoff calculates the wait time before the next retry using exponential backoff with jitter
func (c *Client) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: min * 2^attempt
	backoff := c.retryWaitMin * time.Duration(math.Pow(2, float64(attempt-1)))

	// Cap at max
	if backoff > c.retryWaitMax {
		backoff = c.retryWaitMax
	}

	// Add jitter (±25%)
	jitter := time.Duration(rand.Int63n(int64(backoff / 2)))
	return backoff + jitter
}

// Get performs a GET request
func (c *Client) Get(ctx context.Context, path string, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp, result)
}

// Post performs a POST request
func (c *Client) Post(ctx context.Context, path string, body, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp, result)
}

// Put performs a PUT request
func (c *Client) Put(ctx context.Context, path string, body, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPut, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp, result)
}

// Delete performs a DELETE request
func (c *Client) Delete(ctx context.Context, path string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// For DELETE, we don't expect a body, just check status
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp)
	}

	return nil
}

// Patch performs a PATCH request
func (c *Client) Patch(ctx context.Context, path string, body, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPatch, path, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return c.handleResponse(resp, result)
}

// handleResponse processes the HTTP response
func (c *Client) handleResponse(resp *http.Response, result interface{}) error {
	// Check for error status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return parseAPIError(resp)
	}

	// If no result is expected, return early
	if result == nil {
		return nil
	}

	// Read and parse the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	if len(body) == 0 {
		return nil
	}

	if err := json.Unmarshal(body, result); err != nil {
		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}
