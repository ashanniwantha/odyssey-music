package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// refreshTokenBytes is the raw random material per token, before encoding.
const refreshTokenBytes = 32

// RefreshToken pairs the plaintext, which handed to the client
// with the hash we store in the DB.
type RefreshToken struct {
	PlainText string
	Hash      string
}

// GenerateRefreshToken mints a new refresh token.
func GenerateRefreshToken() (RefreshToken, error) {
	raw := make([]byte, refreshTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		// rand.Read only fails if the OS entrophy source is broken.
		// Which makes the system unsuitable for token issues
		return RefreshToken{}, fmt.Errorf("reading random bytes for refresh token: %w", err)
	}

	plainText := base64.RawURLEncoding.EncodeToString(raw)

	return RefreshToken{
		PlainText: plainText,
		Hash:      HashRefreshToken(plainText),
	}, nil
}

// HashRefreshToken computes the storage hash of a refresh token plaintext.
//
// Called in two places:
//
//  1. During issuance (inside GenerateRefreshToken).
//  2. During validation, by the user service, which receives a plaintext
//     from the client and must hash it before looking up the session row.
func HashRefreshToken(plainText string) string {
	sum := sha256.Sum256([]byte(plainText))
	return hex.EncodeToString(sum[:])
}
