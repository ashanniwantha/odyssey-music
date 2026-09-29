package user

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// ENTITIES
// ============================================================

// User is the domain representation of row in the users table.
type User struct {
	ID             uuid.UUID
	Username       string
	Email          string
	HashedPassword string
	FirstName      *string
	LastName       *string
	DOB            *time.Time
	AllowExplicit  bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// FullName returns "First Last" using whichever name parts are set,
// or the empty string if neither is. Handlers use this for display
// names when a user hasn't chosen one; they fall back to Username.
func (u *User) FullName() string {
	switch {
	case u.FirstName != nil && u.LastName != nil:
		return *u.FirstName + " " + *u.LastName
	case u.FirstName != nil:
		return *u.FirstName
	case u.LastName != nil:
		return *u.LastName
	default:
		return ""
	}
}

// DisplayName is the name to show when referring to this user in a UI:
// the full name if we have one, otherwise the username. Always
// non-empty (username is required).
func (u *User) DisplayName() string {
	if name := u.FullName(); name != "" {
		return name
	}
	return u.Username
}

// Tier mirrors the user_tier enum in the database.
type Tier string

const (
	TierFree    Tier = "free"
	TierPremium Tier = "premium"
	TierArtist  Tier = "artist"
	TierAdmin   Tier = "admin"
)

// Valid reports whether t is one of the known tiers. Useful as a guard
// in the service layer before writing to the DB, and as a cheap sanity
// check when reading back a value that came from outside our process.
func (t Tier) Valid() bool {
	switch t {
	case TierFree, TierPremium, TierArtist, TierAdmin:
		return true
	default:
		return false
	}
}

// Subscription is the domain representation of a row in users_subscriptions.
type Subscription struct {
	UserID    uuid.UUID
	Tier      Tier
	IsActive  bool
	ExpiresAt *time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ============================================================
// DOMAIN ERRORS
// ============================================================
var (
	ErrUserNotFound = errors.New("user not found")

	ErrUsernameTaken = errors.New("username is already taken")

	ErrEmailTaken = errors.New("email is already registered")
)
