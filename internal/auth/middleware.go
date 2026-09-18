package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is an unexported type to avoid collisions in context.
type contextKey string

const (
	userIDCtxKey   contextKey = "userID"
	usernameCtxKey contextKey = "username"
	roleCtxKey     contextKey = "role"
)

// UserIDFromContext retrives the authenticated user's ID from the request context
func UserIDFromContext(r *http.Request) (int64, bool) {
	id, ok := r.Context().Value(userIDCtxKey).(int64)
	return id, ok
}

// UsernameFromContext retrieves the authenticated user's name from the request context.
func UsernameFromContext(r *http.Request) (string, bool) {
	name, ok := r.Context().Value(usernameCtxKey).(string)
	return name, ok
}

// RoleFromContext retrieves the authenticated user's tier/role from the request context.
func RoleFromContext(r *http.Request) (string, bool) {
	role, ok := r.Context().Value(roleCtxKey).(string)
	return role, ok
}

// Authetication returns a middleware that validate an RS256 JWT from the Authorization header,
// extracts user data and inject them into the request context
func Authenticate(tokenGen *TokenGenerator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "missing authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "invalid authorization header format", http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimSpace(parts[1])

			claims := &Claims{}
			claims, err := tokenGen.ValidateAccessToken(tokenStr)
			if err != nil {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			// Inject user details into the context
			ctx := context.WithValue(r.Context(), userIDCtxKey, userID)
			ctx = context.WithValue(ctx, usernameCtxKey, username)
			ctx = context.WithValue(ctx, roleCtxKey, role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
