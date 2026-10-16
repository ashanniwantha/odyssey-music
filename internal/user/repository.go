package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ashanniwantha/odyssey-music/internal/database"
)

// ============================================================
// REPOSITORY INTERFACE
// ============================================================
//
// This is the contract the service layer depends on. It lives in the
// same file as its Postgres implementation on purpose: when the
// interface and the implementation change together (which they do in a
// young codebase), keeping them adjacent makes the diff obvious.
//
// Concrete returns and error semantics are documented per-method. The
// service layer is written against this interface and never imports
// pgx, pgxpool, or anything from internal/database.
type Repository interface {
	// Create inserts a new user and populates ID, CreatedAt and
	// UpdatedAt from the RETURNING clause. The AFTER INSERT trigger on
	// users (trg_users_create_free_subscription) creates a matching row
	// in users_subscriptions, so callers do not insert that separately.
	//
	// Returns ErrUsernameTaken or ErrEmailTaken when a uniqueness
	// constraint fires. Wraps any other error with context.
	Create(ctx context.Context, u *User) error

	// GetByID fetches a user by UUID. Returns ErrUserNotFound if no
	// row matches.
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)

	// GetByEmail fetches a user by email. Comparison is case-
	// insensitive because the column is CITEXT, so callers can pass
	// the email in any case. Returns ErrUserNotFound if no row
	// matches.
	GetByEmail(ctx context.Context, email string) (*User, error)

	// Update writes the mutable profile fields: username, first_name,
	// last_name, dob, allow_explicit. Email and password are handled
	// elsewhere for distinct permission checks and audit trails.
	//
	// Returns ErrUserNotFound if the row is gone, ErrUsernameTaken if
	// the new username collides, or a wrapped error otherwise.
	Update(ctx context.Context, u *User) error

	// UpdatePassword replaces only the hashed_password column. Split
	// from Update so "change your profile" and "change your password"
	// remain separate operations — different audit logs, different
	// confirmation flows, different permission scopes if you ever add
	// them.
	UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error

	// Delete permanently removes a user. Foreign-key cascades wipe
	// their subscriptions, sessions, playlists, likes, saves, follows
	// and play history in the same statement. Returns ErrUserNotFound
	// if no row matched.
	Delete(ctx context.Context, id uuid.UUID) error

	// GetSubscription reads the current subscription row for a user.
	// Every user has exactly one because the AFTER INSERT trigger on
	// users creates it. Absence therefore means the user itself is
	// gone, and we return ErrUserNotFound to match.
	GetSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error)

	// UpdateSubscription writes tier, is_active and expires_at for an
	// existing subscription. Used by both the upgrade and downgrade
	// flows. Returns ErrUserNotFound if the row is gone (which means
	// the user was deleted between the check and the write).
	UpdateSubscription(ctx context.Context, s *Subscription) error
}

// ============================================================
// POSTGRES IMPLEMENTATION
// ============================================================

// postgresRepository is the pgx-backed implementation of Repository.
// Construct it once at startup with NewRepository and share the
// instance process-wide. It is safe for concurrent use because the
// underlying pgxpool.Pool serializes access to connections internally.
type postgresRepository struct {
	pool *pgxpool.Pool
}

// NewRepository wires a pgxpool into a Repository.
func NewRepository(pool *pgxpool.Pool) Repository {
	return &postgresRepository{pool: pool}
}

// conn returns the transaction currently attached to ctx, or the pool
// when no transaction is in play. Every query in this file is issued
// through this helper — which is what lets the service layer wrap a
// series of repository calls in a single transaction without any of
// those calls needing to know they are inside one.
//
// This is the pattern that makes the DBTX type in internal/database
// worth having: the alternative is threading a *pgx.Tx parameter
// through every method signature, which pollutes the interface.
func (r *postgresRepository) conn(ctx context.Context) database.DBTX {
	if tx := database.TxFromContext(ctx); tx != nil {
		return tx
	}
	return r.pool
}

// userColumns is the projection used by every SELECT and RETURNING in
// this file. Kept as a const so a new column added to the users table
// only needs to be listed once, and so scanUser has exactly one thing
// to keep in sync with.
const userColumns = `
	id,
	username,
	email,
	hashed_password,
	first_name,
	last_name,
	dob,
	allow_explicit,
	created_at,
	updated_at
`

// scanUser reads a single users row from a pgx.Row. Centralizing this
// means GetByID, GetByEmail and any future lookup return identical
// fields in identical order, and pgx.ErrNoRows propagates to the
// caller for translation into our own ErrUserNotFound.
//
// Pointer fields (FirstName, LastName, DOB) map to nullable columns.
// pgx v5 scans NULL into a *string / *time.Time as nil — no
// intermediate sql.NullString required.
func scanUser(row pgx.Row) (*User, error) {
	var u User
	err := row.Scan(
		&u.ID,
		&u.Username,
		&u.Email,
		&u.HashedPassword,
		&u.FirstName,
		&u.LastName,
		&u.DOB,
		&u.AllowExplicit,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// mapUserWriteError translates Postgres unique-violation errors into
// the matching domain sentinel. Every write path in this file routes
// through it, so when the schema gains a new unique constraint there
// is exactly one place to add the mapping.
//
// The op string is prepended to any error that isn't a recognized
// constraint violation, so callers get a message like
// "insert user: <pgx error>" and can still errors.Is against the
// wrapped cause if they need to.
func mapUserWriteError(op string, err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		switch pgErr.ConstraintName {
		case "uq_users_username":
			return ErrUsernameTaken
		case "uq_users_email":
			return ErrEmailTaken
		}
	}
	return fmt.Errorf("%s: %w", op, err)
}

// ------------------------------------------------------------
// WRITE OPERATIONS
// ------------------------------------------------------------

// Create inserts a new users row.
//
// The RETURNING clause pulls back the columns the database generates:
// id (from uuidv7()), created_at and updated_at. Without RETURNING we
// would either issue a second SELECT (extra round trip, race window)
// or hand back a User with a zero UUID — both are wrong.
//
// The subscription row is created automatically by the AFTER INSERT
// trigger on users, so callers do not need to insert it. If the
// trigger is ever removed, this method must be updated in lockstep.
func (r *postgresRepository) Create(ctx context.Context, u *User) error {
	const query = `
		INSERT INTO users (
			username,
			email,
			hashed_password,
			first_name,
			last_name,
			dob,
			allow_explicit
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := r.conn(ctx).QueryRow(ctx, query,
		u.Username,
		u.Email,
		u.HashedPassword,
		u.FirstName,
		u.LastName,
		u.DOB,
		u.AllowExplicit,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)

	if err != nil {
		return mapUserWriteError("insert user", err)
	}
	return nil
}

// Update writes the mutable profile fields on an existing user.
//
// Note what is NOT updated here: email, hashed_password. Both live in
// their own methods so the audit trail and permission scopes stay
// distinct.
//
// RETURNING updated_at brings the Go struct's UpdatedAt in sync with
// what the BEFORE UPDATE trigger just wrote to the row. Without it,
// the caller would hold a stale value and any subsequent write using
// that struct would be subtly wrong.
func (r *postgresRepository) Update(ctx context.Context, u *User) error {
	const query = `
		UPDATE users
		SET username       = $2,
		    first_name     = $3,
		    last_name      = $4,
		    dob            = $5,
		    allow_explicit = $6
		WHERE id = $1
		RETURNING updated_at
	`

	err := r.conn(ctx).QueryRow(ctx, query,
		u.ID,
		u.Username,
		u.FirstName,
		u.LastName,
		u.DOB,
		u.AllowExplicit,
	).Scan(&u.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return mapUserWriteError("update user", err)
	}
	return nil
}

// UpdatePassword replaces only the hashed_password column. Uses Exec
// rather than QueryRow because there is nothing to RETURNING — the
// caller (the service) has no use for the row's generated values
// after a password change, and Exec is cheaper.
//
// RowsAffected tells us whether the row existed. If zero rows were
// affected, the user was deleted between our read and our write, so
// we surface ErrUserNotFound rather than silently succeeding.
func (r *postgresRepository) UpdatePassword(ctx context.Context, id uuid.UUID, hashedPassword string) error {
	const query = `
		UPDATE users
		SET hashed_password = $2
		WHERE id = $1
	`

	tag, err := r.conn(ctx).Exec(ctx, query, id, hashedPassword)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// Delete removes a user row. All of the user's dependent rows
// (subscription, sessions, playlists, likes, saves, follows, play
// history) are removed by ON DELETE CASCADE in the same statement —
// the database does the fan-out, not application code, so there is no
// way for a partial delete to leave orphans behind.
func (r *postgresRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM users WHERE id = $1`

	tag, err := r.conn(ctx).Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrUserNotFound
	}
	return nil
}

// ------------------------------------------------------------
// READ OPERATIONS
// ------------------------------------------------------------

// GetByID fetches a user by primary key. The most common lookup in
// the codebase — used by the auth middleware on every request once
// we add per-request user hydration.
func (r *postgresRepository) GetByID(ctx context.Context, id uuid.UUID) (*User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE id = $1`

	u, err := scanUser(r.conn(ctx).QueryRow(ctx, query, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return u, nil
}

// GetByEmail fetches a user by email. The comparison is case-
// insensitive because the column is CITEXT — callers should pass the
// email exactly as the user typed it; Postgres normalizes on the way
// in and on the way out.
//
// Used by the login flow, which needs the hashed password from the
// same row as the identity. That is why User carries HashedPassword.
func (r *postgresRepository) GetByEmail(ctx context.Context, email string) (*User, error) {
	query := `SELECT ` + userColumns + ` FROM users WHERE email = $1`

	u, err := scanUser(r.conn(ctx).QueryRow(ctx, query, email))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return u, nil
}

// ------------------------------------------------------------
// SUBSCRIPTION
// ------------------------------------------------------------

// GetSubscription reads the users_subscriptions row for a user. Every
// user has one because of the AFTER INSERT trigger on users, so a
// missing row means the user was deleted — hence ErrUserNotFound.
func (r *postgresRepository) GetSubscription(ctx context.Context, userID uuid.UUID) (*Subscription, error) {
	const query = `
		SELECT user_id, tier, is_active, expires_at, created_at, updated_at
		FROM users_subscriptions
		WHERE user_id = $1
	`

	var s Subscription
	err := r.conn(ctx).QueryRow(ctx, query, userID).Scan(
		&s.UserID,
		&s.Tier,
		&s.IsActive,
		&s.ExpiresAt,
		&s.CreatedAt,
		&s.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrUserNotFound
		}
		return nil, fmt.Errorf("get subscription: %w", err)
	}
	return &s, nil
}

// UpdateSubscription writes tier, is_active and expires_at. Used by
// both upgrade and downgrade flows, plus any future cancellation
// handling (is_active = false, expires_at unchanged so the customer
// retains access until the period ends).
//
// ExpiresAt is a *time.Time because the column is nullable — free
// tier leaves it NULL, paid tiers set it.
func (r *postgresRepository) UpdateSubscription(ctx context.Context, s *Subscription) error {
	const query = `
		UPDATE users_subscriptions
		SET tier       = $2,
		    is_active  = $3,
		    expires_at = $4
		WHERE user_id = $1
		RETURNING updated_at
	`

	err := r.conn(ctx).QueryRow(ctx, query,
		s.UserID,
		s.Tier,
		s.IsActive,
		s.ExpiresAt,
	).Scan(&s.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrUserNotFound
		}
		return fmt.Errorf("update subscription: %w", err)
	}
	return nil
}
