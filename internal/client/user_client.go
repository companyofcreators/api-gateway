package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type MasterProfile struct {
	UserID          string  `json:"user_id"`
	IsActive        bool    `json:"is_active"`
	Description     string  `json:"description"`
	ExperienceYears int     `json:"experience_years"`
	Rating          float64 `json:"rating"`
	CompletedOrders int     `json:"completed_orders"`
	UpdatedAt       string  `json:"updated_at"`
}

type UserResponse struct {
	Profile       *Profile       `json:"profile"`
	MasterProfile *MasterProfile `json:"master_profile,omitempty"`
	Roles         []string       `json:"roles"`
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

func (c *UserClient) GetProfile(userID string, incomingHeaders http.Header) (*UserResponse, error) {
	reqURL := c.baseURL + "/internal/users/" + url.PathEscape(userID)

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	copyHeader(req, incomingHeaders, "X-User-Id")
	copyHeader(req, incomingHeaders, "X-User-Email")
	copyHeader(req, incomingHeaders, "X-User-Role")
	copyHeader(req, incomingHeaders, "X-Signature")
	copyHeader(req, incomingHeaders, "X-Request-ID")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("user-service returned status %d (failed to read body: %w)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("user-service returned status %d: %s", resp.StatusCode, string(body))
	}

	var userResp UserResponse
	if err := json.NewDecoder(resp.Body).Decode(&userResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &userResp, nil
}
