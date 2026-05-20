package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Order represents an order returned by the order-service.
type Order struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	OfferID   string  `json:"offer_id"`
	Status    string  `json:"status"`
	Amount    float64 `json:"amount"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

// OrderClient is an HTTP client for the order-service internal API.
type OrderClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewOrderClient creates a new OrderClient pointed at the given order-service base URL.
func NewOrderClient(baseURL string) *OrderClient {
	return &OrderClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetOrders fetches orders for the given user ID from the order-service.
// It limits results to the 5 most recent orders.
func (c *OrderClient) GetOrders(userID string, incomingHeaders http.Header) ([]Order, error) {
	url := fmt.Sprintf("%s/internal/orders?user_id=%s&limit=5", c.baseURL, userID)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Forward internal headers.
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
		return nil, fmt.Errorf("order-service returned status %d: %s", resp.StatusCode, string(body))
	}

	var orders []Order
	if err := json.NewDecoder(resp.Body).Decode(&orders); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return orders, nil
}
