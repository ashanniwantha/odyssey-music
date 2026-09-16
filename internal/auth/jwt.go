package auth

import (
	"crypto/rsa"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ServerClaims defines strongly-type JWT body
type ServerClaims struct {
	Username string `json:"username"`
	Role     string `json:"role,omitempty"`
	jwt.RegisteredClaims
}

type TokenGenerator struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	expiration time.Duration
}

// NewTokenGenerator intializes the generator using parsed RSA cryptographic keys.
func NewTokenGenerator(
	privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, expiration time.Duration) *TokenGenerator {
	return &TokenGenerator{
		privateKey: privateKey,
		publicKey:  publicKey,
		expiration: expiration,
	}
}

// GenerateToken signs a new token using the Private Key.
func (g *TokenGenerator) GenerateToken(userID int64, username string, role string) (string, error) {
	claims := ServerClaims{
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "odyssey-music-auth",
			Subject:   strconv.FormatInt(userID, 10),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(g.expiration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	// Use RS256
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(g.privateKey)
	if err != nil {
		return "", fmt.Errorf("signing token with private key: %w", err)
	}
	return signed, nil
}

// ValidateToken parses and validate a JWT token using the Public Key.
func (g *TokenGenerator) ValidateToken(tokenString string) (userID int64, username string, role string, err error) {
	token, err := jwt.ParseWithClaims(tokenString, &ServerClaims{}, func(token *jwt.Token) (any, error) {
		// Ensure the signing method
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		// return public key for signature verification
		return g.publicKey, nil
	})
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid token: %w", err)
	}

	claims, ok := token.Claims.(*ServerClaims)
	if !ok || !token.Valid {
		return 0, "", "", fmt.Errorf("invalid token claims")
	}

	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid user id format: %w", err)
	}

	return id, claims.Username, claims.Role, nil
}
