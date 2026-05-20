package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Profile struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	AvatarURL string `json:"avatar_url"`
	Phone     string `json:"phone"`
	Birthdate string `json:"birthdate"`
	UpdatedAt string `json:"updated_at"`
}

type UserResponse struct {
	Profile *Profile `json:"profile"`
	Roles   []string `json:"roles"`
}

type UserClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewUserClient(baseURL string) *UserClient {
	return &UserClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *UserClient) GetProfile(userID string, incomingHeaders http.Header) (*Profile, error) {
	url := fmt.Sprintf("%s/internal/users/%s", c.baseURL, userID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

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

	var userResp UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return userResp.Profile, nil
}
