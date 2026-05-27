package auth

import (
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	jwt.RegisteredClaims
	Email string   `json:"email"`
	Roles []string `json:"roles"`
}

// RoleString returns the primary role for X-User-Role header.
// Uses the first role from the JWT roles array (highest privilege).
func (c Claims) RoleString() string {
	if len(c.Roles) > 0 {
		return c.Roles[0]
	}
	return ""
}
