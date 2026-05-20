package auth

import (
	"crypto/rsa"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// JWTValidator validates RS256-signed JWTs using a public key.
// The gateway does NOT call the auth service per request —
// it validates tokens locally using the pre-loaded public key.
type JWTValidator struct {
	publicKey *rsa.PublicKey
}

// NewJWTValidator loads an RSA public key from the given PEM file path
// and returns a ready-to-use JWTValidator.
func NewJWTValidator(publicKeyPath string) (*JWTValidator, error) {
	pemBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read public key file %s: %w", publicKeyPath, err)
	}

	key, err := jwt.ParseRSAPublicKeyFromPEM(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse RSA public key from PEM: %w", err)
	}

	return &JWTValidator{
		publicKey: key,
	}, nil
}

// Validate parses and validates an RS256 JWT token string.
// It returns the parsed Claims on success or an error if the token
// is invalid, expired, or uses an unexpected signing method.
func (v *JWTValidator) Validate(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(token *jwt.Token) (interface{}, error) {
			// Enforce RS256 signing algorithm.
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return v.publicKey, nil
		},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuedAt(),
	)

	if err != nil {
		return nil, fmt.Errorf("token validation failed: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token claims")
	}

	return claims, nil
}
