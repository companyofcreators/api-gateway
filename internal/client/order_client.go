package client

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Order represents an order returned by the order-service.
type Order struct {
	ID              string   `json:"id"`
	CustomerID      string   `json:"customer_id"`
	AcceptedOfferID *string  `json:"accepted_offer_id,omitempty"`
	CategoryID      string   `json:"category_id"`
	Status          string   `json:"status"`
	Price           float64  `json:"price"`
	FinalPrice      *float64 `json:"final_price,omitempty"`
	Currency        string   `json:"currency"`
	Address         string   `json:"address"`
	Title           string   `json:"title"`
	Description     string   `json:"description"`
	CreatedAt       string   `json:"created_at"`
	UpdatedAt       string   `json:"updated_at"`
}

type listOrdersResponse struct {
	Orders []Order `json:"orders"`
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
	u, err := url.Parse(c.baseURL + "/internal/orders")
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}
	q := u.Query()
	q.Set("user_id", userID)
	q.Set("limit", "5")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Forward internal headers.
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
			return nil, fmt.Errorf("order-service returned status %d (failed to read body: %w)", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("order-service returned status %d: %s", resp.StatusCode, string(body))
	}

	var result listOrdersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Orders, nil
}
