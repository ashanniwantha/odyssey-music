# Odyssey Music

Odyssey Music is an open-source backend experiment for a digital streaming
platform (DSP), inspired by products such as Spotify and Apple Music. It is
being built in Go with PostgreSQL and Redis, with a focus on clear domain
boundaries, token-based authentication, and a catalog/library data model that
can grow into a full music service.

> **Project status:** early development. The database schema and foundational
> packages are in place, but the HTTP API, application services, migration
> runner, and streaming workflows are not complete yet.

## Current foundations

- Go API process using `net/http` and `go-chi/chi`
- PostgreSQL connection pooling through `pgx/v5`
- Redis client initialization and health checks
- Structured logging with `log/slog`
- Configuration through environment variables and an optional `.env` file
- RSA-signed access tokens using RS256 and validated issuer, algorithm,
  expiration, user ID, and session ID claims
- Cryptographically random refresh tokens stored as SHA-256 hashes
- Authentication middleware that extracts user and session IDs into request
  context
- User domain model and PostgreSQL repository operations for:
  - user creation, lookup, profile updates, password updates, and deletion
  - subscription lookup and updates
- Artist domain model and service/repository contracts
- PostgreSQL schema covering users, sessions, subscriptions, labels, artists,
  albums, tracks, genres, playlists, user libraries, follows, play history,
  and audio features
- Docker Compose definitions for the API, PostgreSQL, and Redis
- Multi-stage Docker image build

## Planned DSP capabilities

The schema is designed to support the following product areas as the API is
implemented:

- account registration, login, refresh, logout, and password recovery
- free, premium, artist, and administrator tiers
- artist, label, album, track, genre, and catalog management
- playlist ownership, collaboration, and public/private playlists
- liked tracks, saved albums, followed artists, and followed playlists
- play history, play counts, and album statistics
- search over artists, albums, and tracks
- audio file delivery and playback authorization
- recommendations and personalized discovery
- administrative and artist tooling

These are design targets, not currently available API endpoints.

## Technology stack

- **Language:** Go 1.26+
- **HTTP:** `net/http`, `go-chi/chi`
- **Database:** PostgreSQL 18, `pgx/v5`
- **Cache/session support:** Redis 7
- **Authentication:** RSA/RS256 JWT access tokens and hashed refresh tokens
- **Containers:** Docker and Docker Compose

## Repository layout

```text
cmd/api/                 API process entrypoint
internal/auth/           JWT, refresh-token, and auth middleware foundations
internal/artist/         Artist domain types and contracts
internal/config/         Environment-based application configuration
internal/database/       PostgreSQL pool and transaction abstractions
internal/logger/         Environment-aware slog setup
internal/redis/          Redis client initialization
internal/user/           User domain model and PostgreSQL repository
migrations/              Goose-compatible initial PostgreSQL schema
compose.yaml             Development API, PostgreSQL, and Redis services
compose.test.yaml        Test PostgreSQL and Redis services
Dockerfile               Production-style multi-stage image build
```

## Prerequisites

- Go 1.26 or newer
- Docker Engine with Docker Compose
- OpenSSL (for generating local RSA keys if authentication is enabled)

## Configuration

Start with the provided example:

```bash
cp .env.example .env
```

At minimum, set these values in `.env`:

```dotenv
POSTGRES_USER=odyssey
POSTGRES_PASSWORD=change-me
POSTGRES_DB=odyssey
POSTGRES_HOST_PORT=5432
REDIS_HOST_PORT=6379

APP_ENV=development
APP_PORT=8080

JWT_SECRET=replace-this-with-a-long-random-value
JWT_EXPIRATION=5m
```

The application also supports the database pool and Redis settings listed in
`.env.example`. Never commit `.env`, private keys, or production credentials.

## Run with Docker Compose

Start PostgreSQL and Redis:

```bash
docker compose up -d postgres redis
```

Build and start the full stack:

```bash
docker compose up --build
```

The API is configured to listen on `http://localhost:8080` by default. The
current entrypoint initializes PostgreSQL and Redis before starting the HTTP
server. It does not yet expose a completed route set or apply migrations
automatically.

Stop the stack:

```bash
docker compose down
```

To remove local database and Redis volumes as well:

```bash
docker compose down -v
```

## Database migrations

The initial schema is in
`migrations/20260916091720_init_schema.sql` and uses Goose-style
`-- +goose Up` markers. Apply it with the migration tool used by your
environment, or with a PostgreSQL client after reviewing the SQL.

For example, with a local PostgreSQL client:

```bash
psql "$DATABASE_URL" \
  --file migrations/20260916091720_init_schema.sql
```

The schema requires the `citext` and `pg_trgm` PostgreSQL extensions and
defines UUID, catalog, playlist, library, and listening-history relationships.

## Run locally

After PostgreSQL and Redis are available and the environment is configured:

```bash
go mod download
go run ./cmd/api
```

Build the API binary:

```bash
go build -o bin/odyssey-api ./cmd/api
```

## Testing

The repository currently contains the test-service Compose definition but no
committed Go test suite. The intended test dependencies can be started with:

```bash
docker compose -f compose.test.yaml up -d
go test ./...
docker compose -f compose.test.yaml down
```

As the API grows, tests should cover domain validation, repository behavior,
authentication claims and middleware, HTTP handlers, and integration flows
against PostgreSQL and Redis.

## Development roadmap

1. Add a migration runner and make startup/migration behavior explicit.
2. Separate HTTP handlers, application services, and repositories for users,
   authentication, artists, catalog, playlists, and library features.
3. Wire the existing authentication foundations into registration, login,
   refresh, logout, and protected routes.
4. Add request validation, consistent response/error envelopes, and health
   endpoints.
5. Implement catalog ingestion and metadata management.
6. Implement playlist and personal-library workflows.
7. Add object storage/CDN integration for audio and cover art; the database
   currently stores paths rather than serving media.
8. Add search, recommendations, analytics, rate limiting, observability, and
   API documentation.
9. Add unit, integration, and end-to-end CI coverage.

## Important next steps

This project is intentionally unfinished. Before treating it as a usable
backend, the following gaps need to be addressed:

- `cmd/api` currently creates an empty router, so there are no usable API
  endpoints yet.
- The access-token generator and RSA key loading are not wired into startup.
- The configured `JWT_SECRET` is validated by configuration but is not yet
  used by the RSA token implementation.
- There is no application/service layer coordinating repositories and
  transactions.
- Migrations are not run automatically.
- No audio storage, transcoding, streaming, CDN, or signed media URL flow is
  implemented.
- Production secrets and RSA key material must be managed outside the
  repository and rotated safely.

## Contributing

Keep changes focused on one domain or infrastructure concern, preserve the
repository's package boundaries, and add tests with new behavior. Run
`gofmt`, `go vet`, and `go test ./...` before opening a pull request.

## License

No license has been declared yet. Add a license before distributing or
accepting external contributions.
