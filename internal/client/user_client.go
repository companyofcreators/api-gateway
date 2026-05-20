package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Profile represents a user profile returned by the user-service.
type Profile struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// UserClient is an HTTP client for the user-service internal API.
type UserClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewUserClient creates a new UserClient pointed at the given user-service base URL.
func NewUserClient(baseURL string) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetProfile fetches the user profile for the given user ID from the user-service.
// It forwards the X-User-Id, X-User-Email, and X-User-Role headers from the
// incoming request so the user-service can perform its own authorization.
func (c *UserClient) GetProfile(userID string, incomingHeaders http.Header) (*Profile, error) {
	url := fmt.Sprintf("%s/api/v1/users/%s", c.baseURL, userID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Forward internal headers so the user-service trusts the caller.
	copyHeader(req, incomingHeaders, "X-User-Id")
	copyHeader(req, incomingHeaders, "X-User-Email")
	copyHeader(req, incomingHeaders, "X-User-Role")
	copyHeader(req, incomingHeaders, "X-Request-ID")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("user-service returned status %d: %s", resp.StatusCode, string(body))
	}

	var profile Profile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &profile, nil
}

