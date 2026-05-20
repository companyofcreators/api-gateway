package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

// Claims represents the JWT claims for the API gateway.
// It embeds jwt.RegisteredClaims and adds custom fields
// for user identity and authorization.
type Claims struct {
	jwt.RegisteredClaims

	// Sub is the user UUID (embedded in RegisteredClaims.Subject).
	// Email is the user's email address.
	Email string `json:"email"`

	// Role is the user's role (e.g., "user", "moderator", "admin").
	Role string `json:"role"`
}
