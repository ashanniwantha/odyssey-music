package user

import (
	"context"

	"github.com/ashanniwantha/odyssey-music/internal/database"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ===========================================================
// RESPOSITORY INTERFACE
// ===========================================================
//
// This is the contract the service layer depends on.
type Repository interface {
	// Create inserts a new user and populates ID, CreatedAt and
	// UpdatedAt from the RETURNING clause. The AFTER INSERTS trigger on
	// users (trg_users_create_free_subscription) creates a matching row
	// in users_subscriptions, so the callers do not insert that seperately.
	//
	// Returns ErrUsernameTaken of ErrEmailTaken when a uniqueness
	// constraint from DB fires. Wraps any other error with context
	Create(ctx context.Context, u *User) error

	// GetByID fetches a user by UUID.
	// Returns ErrUserNotFound if no row matches.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	// GetByEmail fetches a user by Email.
	// Comparison is case-insensitive because the column is CITEXT.
	// Returns ErrUserNotFound if no row matches.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// Update re-writes the mutables fields of a user:
	// username, first_name, last_name, dob, allow_explicit.
	// email and hashed_password are handled elsewhere,
	// for distinct permission checks and audit trails.
	//
	// Returns ErrUserNotFound if no row matches,
	// ErrUsernameTakes if the new username collides,
	// or otherwise a wrapped error with context.
	Update(ctx context.Context, u *User) error

	// UpdatePassword replaces only the hashed_password column.
	// Which allows two non-conflicting split updates as:
	// 'change your profile' and 'change your password'
	UpdatePassword(ctx context.Context, id uuid.UUID, hashed_password string) error

	// Delete permanently removes a user, foreign key cascade wipes their
	// subscriptions, sessions, playlists, likes, saves, follows and play history
	// in the same operation.
	// Returns ErrUserNotFound if no row matched.
	Delete(ctx context.Context, id uuid.UUID) error

	// GetSubscription reads the current subscription row for a givesn user.
	// Every user has exactly one entry because the AFTER INSERT trigger in
	// users creates it. Absense of it means the user is gone as well, and
	// we return ErrUserNotFound to match.
	GetSubscription(ctx context.Context, id uuid.UUID) (*Subscription, error)

	// UpdateSubscription writes tiers, is_active, expires_at for an
	// existing subscription of a user. Used by both upgrade and downgrade
	// flows. Returns ErrUserNotFound if the row is gone.
	UpdateSubscription(ctx context.Context, id uuid.UUID) error
}

// ===========================================================
// POSTGRES IMPLEMENTATION
// ===========================================================
//
// postgresRepository is the pgx-backed implementation of Repository.
// Construct it once at startup with NewRespository and share the
// instance process-wide. It is safe for concurrent use because the
// underlying pgxpool.pool serializes access to connections internally.
type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository wires a pgxpool into a Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

// conn returns the transaction currently attached to ctx, or the pool when no transaction going on.
// Every query in this file is issues through this helper - which is what lets the service layer
// wrap a series of respository calls in a single transaction without any of those calls needing to
// know they are inside one. This is made possible by the DBXT type internal/database.
func (r *postgresRepository) conn(ctx context.Context) database.DBTX {
	if tx := database.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}
