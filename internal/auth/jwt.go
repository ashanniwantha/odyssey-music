package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Issuer is stamped into every token we sign and verified on every token we accept
const Issuer = "odyssey-music-auth"

// Sentinal error we return by ValidateAccessToken for every single error
var ErrInvalidToken = errors.New("invalid access token")

// Claims defines strongly-type JWT body
type Claims struct {
	jwt.RegisteredClaims
}

// UseID parses the Subject claim back into an UUID.
func (c *Claims) UserID() (uuid.UUID, error) {
	return uuid.Parse(c.Subject)
}

// SessionID parses the jti claim back into an UUID.
func (c *Claims) SessionID() (uuid.UUID, error) {
	return uuid.Parse(c.ID)
}

// TokenGenerator signs and verifies access tokens with an RSA key pair.
type TokenGenerator struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	expiration time.Duration
}

// NewTokenGenerator intializes the generator using parsed RSA cryptographic keys.
func NewTokenGenerator(
	privateKey *rsa.PrivateKey, publicKey *rsa.PublicKey, expiration time.Duration,
) *TokenGenerator {
	return &TokenGenerator{
		privateKey: privateKey,
		publicKey:  publicKey,
		expiration: expiration,
	}
}

// GenerateToken signs a new token using the Private Key for the user and session.
func (g *TokenGenerator) GenerateToken(userID, sessionID uuid.UUID) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    Issuer,
			Subject:   userID.String(),
			ID:        sessionID.String(),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(g.expiration)),
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
func (g *TokenGenerator) ValidateAccessToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	_, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(t *jwt.Token) (any, error) {
			return g.publicKey, nil
		},
		// Pin the accepted algorithm list. Without this, jwt v5 will accept
		// whatever the "alg" header claims, which is the classic algorithm-
		// confusion vulnerability.
		jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}),
		// Reject tokens from a different issuer (e.g. a future sibling service).
		jwt.WithIssuer(Issuer),
		// Refuse tokens with no exp at all. Without this, a token without exp
		// is treated as "never expires" and is impossible to age out.
		jwt.WithExpirationRequired(),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	// the subject and jti must be usable as UUIDs before we hand the claims to anyone downstream.
	if _, err := claims.UserID(); err != nil {
		return nil, fmt.Errorf("%w: malformed subject", ErrInvalidToken)
	}
	if _, err := claims.SessionID(); err != nil {
		return nil, fmt.Errorf("%w: malformed session ID", ErrInvalidToken)
	}

	return claims, nil
}
