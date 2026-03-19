package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// APIError represents an error returned by the CubePath API
type APIError struct {
	StatusCode int
	Message    string
	Detail     string
}

// Error implements the error interface
func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("API Error (HTTP %d): %s - %s", e.StatusCode, e.Message, e.Detail)
	}
	return fmt.Sprintf("API Error (HTTP %d): %s", e.StatusCode, e.Message)
}

// IsNotFound returns true if the error is a 404 Not Found
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsConflict returns true if the error is a 409 Conflict
func (e *APIError) IsConflict() bool {
	return e.StatusCode == http.StatusConflict
}

// IsRateLimited returns true if the error is a 429 Too Many Requests
func (e *APIError) IsRateLimited() bool {
	return e.StatusCode == http.StatusTooManyRequests
}

// IsBadRequest returns true if the error is a 400 Bad Request
func (e *APIError) IsBadRequest() bool {
	return e.StatusCode == http.StatusBadRequest
}

// IsServerError returns true if the error is a 5xx server error
func (e *APIError) IsServerError() bool {
	return e.StatusCode >= 500 && e.StatusCode < 600
}

// parseAPIError parses an HTTP response into an APIError
func parseAPIError(resp *http.Response) error {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    http.StatusText(resp.StatusCode),
			Detail:     fmt.Sprintf("failed to read error response: %v", err),
		}
	}

	// Try to parse FastAPI error format: {"detail": "error message"}
	var apiErr struct {
		Detail string `json:"detail"`
	}

	if err := json.Unmarshal(body, &apiErr); err == nil && apiErr.Detail != "" {
		return &APIError{
			StatusCode: resp.StatusCode,
			Message:    http.StatusText(resp.StatusCode),
			Detail:     apiErr.Detail,
		}
	}

	// Fallback to raw body if JSON parsing fails
	return &APIError{
		StatusCode: resp.StatusCode,
		Message:    http.StatusText(resp.StatusCode),
		Detail:     string(body),
	}
}
