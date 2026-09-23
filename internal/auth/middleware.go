package auth

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// contextKey is an unexported type to avoid collisions in context.
type contextKey struct {
	name string
}

var (
	userIDKey    = contextKey{"userID"}
	sessionIDKey = contextKey{"sessionID"}
)

// UserIDFromContext returns the authenticated user's UUID, if the request
// passed through Authenticate. The bool is false when the middleware did
// not run, which should only happen on routes that were explicitly mounted
// outside the protected group.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

// SessionIDFromContext returns the UUID of the user_sessions row that this
// access token belongs to. Callers that need to revoke (logout) or re-check
// session liveness use this to find the right row.
func SessionIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(sessionIDKey).(uuid.UUID)
	return id, ok
}

// Authenticate returns an http middleware that:
//
//  1. Reads the Authorization header and requires "Bearer <jwt>".
//  2. Validates the JWT via TokenGenerator.ValidateAccessToken.
//  3. Injects the user UUID and session UUID into the request context.
//  4. Calls the next handler.
func Authenticate(tokenGen *TokenGenerator, logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				unauthorized(w, "missing_token", "authorization header is required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				unauthorized(w, "malformed_header", "authorization header muste be 'Bearer <token>'")
				return
			}

			tokenStr := strings.TrimSpace(parts[1])
			if tokenStr == "" {
				unauthorized(w, "missing_token", "bearer token is empty")
				return
			}

			claims, err := tokenGen.ValidateAccessToken(tokenStr)
			if err != nil {
				if errors.Is(err, ErrInvalidToken) {
					logger.Warn("rejected access token", "err", err)
				} else {
					logger.Error("unexpected error validation access token", "err", err)
				}
				unauthorized(w, "invalid_token", "token is invalid or expired")
				return
			}

			// ValidateAccessToken already checked for following userID and sessionID
			// hitting it here means the validation path changed and this is a bug in
			// our own code. Log at Error so it stands out
			userID, err := claims.UserID()
			if err != nil {
				logger.Error("claims passed validation but user id parse failed")

				unauthorized(w, "invalid_token", "token is invalid or expired")
				return
			}

			sessionID, err := claims.SessionID()
			if err != nil {
				logger.Error("claims passes validation but session id parse failed")

				unauthorized(w, "invalid_token", "token is invalid or expired")
				return
			}

			// Inject user details into the context
			ctx := context.WithValue(r.Context(), userIDKey, userID)
			ctx = context.WithValue(ctx, sessionIDKey, sessionID)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// unauthorized writes a 401 with the standard JSON error envelope and the
// WWW-Authenticate header required by RFC 6750.
//
// The `code` is a short machine-readable string the client can branch on
// (e.g. "missing_token" → redirect to login, "invalid_token" → refresh and
// retry). The `message` is human-readable and safe to display.
func unauthorized(w http.ResponseWriter, code, message string) {
	if code == "invalid_token" {
		w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
	} else {
		w.Header().Set("WWW-Authenticate", `Bearer`)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
