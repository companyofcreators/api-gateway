package aggregator

import (
	"encoding/json"
	"net/http"

	"github.com/companyofcreators/api-gateway/internal/client"
	"github.com/gookit/slog"
)

// ProfileResponse is the combined response returned by the aggregator.
// It includes the user's profile from the user-service and their
// recent orders from the order-service.
type ProfileResponse struct {
	Profile *client.Profile `json:"profile"`
	Orders  []client.Order  `json:"orders"`
}

// UserProfileHandler returns an http.Handler that aggregates user data
// from multiple microservices:
//   - Fetches profile information from the user-service
//   - Fetches the user's 5 most recent orders from the order-service
//   - Returns a combined JSON response
//
// The user ID is read from the X-User-Id header set by the Auth middleware.
func UserProfileHandler(userClient *client.UserClient, orderClient *client.OrderClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-Id")
		if userID == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"error": "user not authenticated",
			})
			return
		}

		incomingHeaders := r.Header

		// Fetch profile from user-service.
		profile, err := userClient.GetProfile(userID, incomingHeaders)
		if err != nil {
			slog.Error("failed to fetch user profile",
				"error", err,
				"user_id", userID,
			)
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": "failed to fetch user profile",
			})
			return
		}

		// Fetch recent orders from order-service.
		orders, err := orderClient.GetOrders(userID, incomingHeaders)
		if err != nil {
			slog.Warn("failed to fetch user orders",
				"error", err,
				"user_id", userID,
			)
			// Orders are optional — don't fail the entire request.
			orders = []client.Order{}
		}

		response := ProfileResponse{
			Profile: profile,
			Orders:  orders,
		}

		writeJSON(w, http.StatusOK, response)
	}
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
