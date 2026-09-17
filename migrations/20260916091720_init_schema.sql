-- +goose Up
-- +goose StatementBegin

-- ============================================================
-- EXTENSIONS
-- ============================================================
CREATE EXTENSION IF NOT EXISTS citext;
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- ============================================================
-- ENUM TYPES
-- ============================================================
CREATE TYPE artist_track_role AS ENUM ('main', 'feature', 'producer', 'remixer', 'composer', 'lyricist');
CREATE TYPE user_tier         AS ENUM ('free', 'premium', 'artist', 'admin');
CREATE TYPE album_type        AS ENUM ('album', 'single', 'ep', 'compilation');

-- ============================================================
-- SHARED TRIGGER FUNCTION: auto-update updated_at
-- ============================================================
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ============================================================
-- USERS
-- ============================================================
CREATE TABLE IF NOT EXISTS users (
    id              UUID        PRIMARY KEY DEFAULT uuidv7(),
    username        CITEXT      NOT NULL,
    email           CITEXT      NOT NULL,
    hashed_password TEXT        NOT NULL,
    first_name      TEXT,
    last_name       TEXT,
    dob             DATE,
    allow_explicit  BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_users_username UNIQUE (username),
    CONSTRAINT uq_users_email    UNIQUE (email),
    CONSTRAINT chk_users_username_not_empty CHECK (length(trim(username::text)) > 0),
    CONSTRAINT chk_users_email_not_empty    CHECK (length(trim(email::text))    > 0)
);

CREATE TRIGGER trg_users_updated_at
    BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- AUTH
-- ============================================================
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id    UUID        NOT NULL,
    token_hash TEXT        NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_password_reset_tokens_hash UNIQUE (token_hash),
    CONSTRAINT fk_password_reset_tokens_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_sessions (
    id                 UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id            UUID        NOT NULL,
    refresh_token_hash TEXT        NOT NULL,
    user_agent         TEXT,
    ip_address         INET,
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_user_sessions_token UNIQUE (refresh_token_hash),
    CONSTRAINT fk_user_sessions_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE
);

CREATE TRIGGER trg_user_sessions_updated_at
    BEFORE UPDATE ON user_sessions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- SUBSCRIPTIONS
-- ------------------------------------------------------------
-- Presence is MANDATORY: every user gets a row on signup.
-- Enforced by the trigger below (see USERS <- SUBSCRIPTIONS).
-- ============================================================
CREATE TABLE IF NOT EXISTS users_subscriptions (
    user_id    UUID        PRIMARY KEY,
    tier       user_tier   NOT NULL DEFAULT 'free',
    is_active  BOOLEAN     NOT NULL DEFAULT TRUE,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_users_subscriptions_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE
);

CREATE TRIGGER trg_users_subscriptions_updated_at
    BEFORE UPDATE ON users_subscriptions
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- AUTO-CREATE FREE SUBSCRIPTION ON USER SIGNUP
-- ============================================================
CREATE OR REPLACE FUNCTION create_free_subscription_for_user()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO users_subscriptions (user_id, tier)
    VALUES (NEW.id, 'free');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trg_users_create_free_subscription
    AFTER INSERT ON users
    FOR EACH ROW EXECUTE FUNCTION create_free_subscription_for_user();

-- ============================================================
-- CATALOG: RECORD LABELS
-- ============================================================
CREATE TABLE IF NOT EXISTS record_labels (
    id            UUID        PRIMARY KEY DEFAULT uuidv7(),
    name          TEXT        NOT NULL,
    country       TEXT,
    contact_email CITEXT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_record_labels_name UNIQUE (name),
    CONSTRAINT chk_record_labels_name_not_empty CHECK (length(trim(name)) > 0)
);

CREATE TRIGGER trg_record_labels_updated_at
    BEFORE UPDATE ON record_labels
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- CATALOG: ARTISTS
-- ============================================================
CREATE TABLE IF NOT EXISTS artists (
    id          UUID        PRIMARY KEY DEFAULT uuidv7(),
    name        TEXT        NOT NULL,
    description TEXT,
    country     TEXT,
    image_url   TEXT,
    label_id    UUID,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_artists_record_labels FOREIGN KEY (label_id)
        REFERENCES record_labels(id) ON DELETE SET NULL,
    CONSTRAINT chk_artists_name_not_empty CHECK (length(trim(name)) > 0)
);

CREATE TRIGGER trg_artists_updated_at
    BEFORE UPDATE ON artists
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- CATALOG: ALBUMS
-- ============================================================
CREATE TABLE IF NOT EXISTS albums (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    name         TEXT        NOT NULL,
    album_type   album_type  NOT NULL DEFAULT 'album',
    release_date DATE        NOT NULL,
    label_id     UUID,
    cover_art    TEXT        NOT NULL,
    play_count   BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_albums_record_labels FOREIGN KEY (label_id)
        REFERENCES record_labels(id) ON DELETE SET NULL,
    CONSTRAINT chk_albums_name_not_empty      CHECK (length(trim(name))      > 0),
    CONSTRAINT chk_albums_cover_art_not_empty CHECK (length(trim(cover_art)) > 0),
    CONSTRAINT chk_albums_play_count_non_neg  CHECK (play_count >= 0)
);

CREATE TRIGGER trg_albums_updated_at
    BEFORE UPDATE ON albums
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- CATALOG: GENRES
-- ============================================================
CREATE TABLE IF NOT EXISTS genres (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    name       CITEXT      NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT uq_genres_name UNIQUE (name),
    CONSTRAINT chk_genres_name_not_empty CHECK (length(trim(name::text)) > 0)
);

CREATE TABLE IF NOT EXISTS albums_genres (
    album_id UUID NOT NULL,
    genre_id UUID NOT NULL,

    CONSTRAINT pk_albums_genres PRIMARY KEY (album_id, genre_id),
    CONSTRAINT fk_albums_genres_albums FOREIGN KEY (album_id)
        REFERENCES albums(id) ON DELETE CASCADE,
    CONSTRAINT fk_albums_genres_genres FOREIGN KEY (genre_id)
        REFERENCES genres(id) ON DELETE CASCADE
);

-- ============================================================
-- CATALOG: TRACKS
-- NOTE: Soft-delete is kept ONLY here. Tracks are hidden, not
-- destroyed, so their play history, likes, and playlist
-- references survive.
-- ============================================================
CREATE TABLE IF NOT EXISTS tracks (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    name         TEXT        NOT NULL,
    album_id     UUID        NOT NULL,
    track_number INT         NOT NULL DEFAULT 1,
    disc_number  INT         NOT NULL DEFAULT 1,
    duration_ms  INT         NOT NULL,
    storage_path TEXT        NOT NULL,
    is_explicit  BOOLEAN     NOT NULL DEFAULT FALSE,
    lyrics       TEXT,
    play_count   BIGINT      NOT NULL DEFAULT 0,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at   TIMESTAMPTZ,

    CONSTRAINT fk_tracks_albums FOREIGN KEY (album_id)
        REFERENCES albums(id) ON DELETE CASCADE,
    CONSTRAINT uq_tracks_album_position UNIQUE (album_id, disc_number, track_number),
    CONSTRAINT chk_tracks_name_not_empty         CHECK (length(trim(name))         > 0),
    CONSTRAINT chk_tracks_storage_path_not_empty CHECK (length(trim(storage_path)) > 0),
    CONSTRAINT chk_tracks_duration_positive      CHECK (duration_ms > 0 AND duration_ms < 86400000),
    CONSTRAINT chk_tracks_disc_positive          CHECK (disc_number  > 0),
    CONSTRAINT chk_tracks_number_positive        CHECK (track_number > 0),
    CONSTRAINT chk_tracks_play_count_non_neg     CHECK (play_count >= 0)
);

CREATE TRIGGER trg_tracks_updated_at
    BEFORE UPDATE ON tracks
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- PLAYLISTS
-- NOTE: playlists.owner_id is the SINGLE source of truth for
-- ownership. The users_playlists junction tracks collaborators
-- ONLY, not the owner.
-- ============================================================
CREATE TABLE IF NOT EXISTS playlists (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    name       TEXT        NOT NULL,
    cover_art  TEXT,
    owner_id   UUID        NOT NULL,
    is_public  BOOLEAN     NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_playlists_users FOREIGN KEY (owner_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT chk_playlists_name_not_empty CHECK (length(trim(name)) > 0)
);

CREATE TRIGGER trg_playlists_updated_at
    BEFORE UPDATE ON playlists
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- JUNCTION: ALBUMS <-> ARTISTS
-- ============================================================
CREATE TABLE IF NOT EXISTS albums_artists (
    album_id  UUID NOT NULL,
    artist_id UUID NOT NULL,

    CONSTRAINT pk_albums_artists PRIMARY KEY (album_id, artist_id),
    CONSTRAINT fk_albums_artists_albums FOREIGN KEY (album_id)
        REFERENCES albums(id) ON DELETE CASCADE,
    CONSTRAINT fk_albums_artists_artists FOREIGN KEY (artist_id)
        REFERENCES artists(id) ON DELETE CASCADE
);

-- ============================================================
-- CONSTRAINT: EVERY ALBUM MUST HAVE AT LEAST ONE ARTIST
-- ------------------------------------------------------------
-- Deferred constraint triggers: the check runs at COMMIT time,
-- so the app can insert the album and its albums_artists rows
-- within the same transaction. If the transaction commits with
-- zero artist rows attached, it aborts.
-- ============================================================
CREATE OR REPLACE FUNCTION ensure_album_has_artist()
RETURNS TRIGGER AS $$
DECLARE
    v_album_id     UUID;
    v_album_exists BOOLEAN;
BEGIN
    IF TG_TABLE_NAME = 'albums' THEN
        v_album_id := NEW.id;
    ELSE
        v_album_id := OLD.album_id;
    END IF;

    -- If the album no longer exists (hard-delete cascade in
    -- progress), skip the check — there is nothing to protect.
    SELECT EXISTS(SELECT 1 FROM albums WHERE id = v_album_id) INTO v_album_exists;
    IF NOT v_album_exists THEN
        RETURN NULL;
    END IF;

    IF NOT EXISTS (SELECT 1 FROM albums_artists WHERE album_id = v_album_id) THEN
        RAISE EXCEPTION 'Album % must have at least one artist', v_album_id
            USING ERRCODE = 'foreign_key_violation';
    END IF;

    RETURN NULL;
END;
$$ LANGUAGE plpgsql;

CREATE CONSTRAINT TRIGGER trg_albums_requires_artist
    AFTER INSERT ON albums
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_album_has_artist();

CREATE CONSTRAINT TRIGGER trg_albums_artists_requires_artist
    AFTER DELETE ON albums_artists
    DEFERRABLE INITIALLY DEFERRED
    FOR EACH ROW EXECUTE FUNCTION ensure_album_has_artist();

-- ============================================================
-- JUNCTION: USERS <-> PLAYLISTS (COLLABORATORS ONLY)
-- ------------------------------------------------------------
-- The playlist owner is playlists.owner_id.
-- This junction tracks NON-OWNER collaborators only.
-- Roles allowed: 'collaborator' (can edit) or 'viewer' (read-only).
-- ============================================================
CREATE TABLE IF NOT EXISTS users_playlists (
    user_id     UUID NOT NULL,
    playlist_id UUID NOT NULL,
    role        TEXT NOT NULL DEFAULT 'collaborator',

    CONSTRAINT pk_users_playlists PRIMARY KEY (user_id, playlist_id),
    CONSTRAINT chk_users_playlists_role CHECK (role IN ('collaborator', 'viewer')),
    CONSTRAINT fk_users_playlists_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_users_playlists_playlists FOREIGN KEY (playlist_id)
        REFERENCES playlists(id) ON DELETE CASCADE
);

-- ============================================================
-- JUNCTION: PLAYLISTS <-> TRACKS
-- No duplicates (composite PK).
-- Fractional ordering via DOUBLE PRECISION track_order.
-- ============================================================
CREATE TABLE IF NOT EXISTS playlists_tracks (
    playlist_id UUID             NOT NULL,
    track_id    UUID             NOT NULL,
    track_order DOUBLE PRECISION NOT NULL,

    CONSTRAINT pk_playlists_tracks PRIMARY KEY (playlist_id, track_id),
    CONSTRAINT chk_playlists_tracks_order_positive CHECK (track_order >= 0),
    CONSTRAINT fk_playlists_tracks_playlists FOREIGN KEY (playlist_id)
        REFERENCES playlists(id) ON DELETE CASCADE,
    CONSTRAINT fk_playlists_tracks_tracks FOREIGN KEY (track_id)
        REFERENCES tracks(id) ON DELETE CASCADE
);

-- ============================================================
-- JUNCTION: TRACKS <-> ARTISTS
-- ============================================================
CREATE TABLE IF NOT EXISTS tracks_artists (
    track_id  UUID              NOT NULL,
    artist_id UUID              NOT NULL,
    role      artist_track_role NOT NULL DEFAULT 'main',

    CONSTRAINT pk_tracks_artists PRIMARY KEY (track_id, artist_id),
    CONSTRAINT fk_tracks_artists_tracks FOREIGN KEY (track_id)
        REFERENCES tracks(id) ON DELETE CASCADE,
    CONSTRAINT fk_tracks_artists_artists FOREIGN KEY (artist_id)
        REFERENCES artists(id) ON DELETE CASCADE
);

-- ============================================================
-- USER LIBRARY: LIKED TRACKS, SAVED ALBUMS, FOLLOWS
-- ============================================================
CREATE TABLE IF NOT EXISTS user_liked_tracks (
    user_id  UUID        NOT NULL,
    track_id UUID        NOT NULL,
    liked_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_user_liked_tracks PRIMARY KEY (user_id, track_id),
    CONSTRAINT fk_user_liked_tracks_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_liked_tracks_tracks FOREIGN KEY (track_id)
        REFERENCES tracks(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_saved_albums (
    user_id  UUID        NOT NULL,
    album_id UUID        NOT NULL,
    saved_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_user_saved_albums PRIMARY KEY (user_id, album_id),
    CONSTRAINT fk_user_saved_albums_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_saved_albums_albums FOREIGN KEY (album_id)
        REFERENCES albums(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_followed_artists (
    user_id     UUID        NOT NULL,
    artist_id   UUID        NOT NULL,
    followed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_user_followed_artists PRIMARY KEY (user_id, artist_id),
    CONSTRAINT fk_user_followed_artists_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_followed_artists_artists FOREIGN KEY (artist_id)
        REFERENCES artists(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS user_followed_playlists (
    user_id     UUID        NOT NULL,
    playlist_id UUID        NOT NULL,
    followed_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_user_followed_playlists PRIMARY KEY (user_id, playlist_id),
    CONSTRAINT fk_user_followed_playlists_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_followed_playlists_playlists FOREIGN KEY (playlist_id)
        REFERENCES playlists(id) ON DELETE CASCADE
);

-- ============================================================
-- PLAY HISTORY
-- ------------------------------------------------------------
-- Policy: HISTORY SURVIVES CATALOG REMOVAL.
-- track_id is nullable and uses ON DELETE SET NULL. When a
-- track is hard-deleted, the history row remains but the
-- reference becomes NULL. Read paths must tolerate NULL.
--
-- play_count on tracks/albums is updated asynchronously from
-- this table via a background worker.
-- ============================================================
CREATE TABLE IF NOT EXISTS user_play_history (
    id           UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id      UUID        NOT NULL,
    track_id     UUID,
    played_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ms_played    INT,
    context_type TEXT,
    context_id   UUID,

    CONSTRAINT fk_user_play_history_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_play_history_tracks FOREIGN KEY (track_id)
        REFERENCES tracks(id) ON DELETE SET NULL,
    CONSTRAINT chk_user_play_history_ms_played     CHECK (ms_played IS NULL OR ms_played >= 0),
    CONSTRAINT chk_user_play_history_ms_played_max CHECK (ms_played IS NULL OR ms_played < 86400000)
);

-- ============================================================
-- WHOLE-ALBUM PLAYS
-- ------------------------------------------------------------
-- Distinct signal from per-track play history: the user pressed
-- play on the album and let it run. Recorded as its own event
-- so albums.play_count reflects true album plays, not the sum
-- of individual track plays.
-- ============================================================
CREATE TABLE IF NOT EXISTS user_album_plays (
    id         UUID        PRIMARY KEY DEFAULT uuidv7(),
    user_id    UUID        NOT NULL,
    album_id   UUID        NOT NULL,
    played_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed  BOOLEAN     NOT NULL DEFAULT FALSE,

    CONSTRAINT fk_user_album_plays_users FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_user_album_plays_albums FOREIGN KEY (album_id)
        REFERENCES albums(id) ON DELETE CASCADE
);

-- ============================================================
-- AUDIO FEATURES
-- ============================================================
CREATE TABLE IF NOT EXISTS track_audio_features (
    track_id         UUID        PRIMARY KEY,
    bpm              NUMERIC(6,2),
    key              INT,
    mode             INT,
    energy           NUMERIC(4,3),
    danceability     NUMERIC(4,3),
    valence          NUMERIC(4,3),
    acousticness     NUMERIC(4,3),
    instrumentalness NUMERIC(4,3),
    liveness         NUMERIC(4,3),
    loudness         NUMERIC(6,2),
    speechiness      NUMERIC(4,3),
    time_signature   INT,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_track_audio_features_tracks FOREIGN KEY (track_id)
        REFERENCES tracks(id) ON DELETE CASCADE,
    CONSTRAINT chk_taf_key_range      CHECK (key IS NULL OR (key >= 0 AND key <= 11)),
    CONSTRAINT chk_taf_mode_range     CHECK (mode IS NULL OR mode IN (0, 1)),
    CONSTRAINT chk_taf_energy_range   CHECK (energy IS NULL OR (energy BETWEEN 0 AND 1)),
    CONSTRAINT chk_taf_dance_range    CHECK (danceability IS NULL OR (danceability BETWEEN 0 AND 1)),
    CONSTRAINT chk_taf_valence_range  CHECK (valence IS NULL OR (valence BETWEEN 0 AND 1)),
    CONSTRAINT chk_taf_acoustic_range CHECK (acousticness IS NULL OR (acousticness BETWEEN 0 AND 1)),
    CONSTRAINT chk_taf_instr_range    CHECK (instrumentalness IS NULL OR (instrumentalness BETWEEN 0 AND 1)),
    CONSTRAINT chk_taf_live_range     CHECK (liveness IS NULL OR (liveness BETWEEN 0 AND 1)),
    CONSTRAINT chk_taf_speech_range   CHECK (speechiness IS NULL OR (speechiness BETWEEN 0 AND 1))
);

CREATE TRIGGER trg_track_audio_features_updated_at
    BEFORE UPDATE ON track_audio_features
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- ============================================================
-- INDEXES
-- ============================================================

-- --- Auth ---
CREATE INDEX idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
CREATE INDEX idx_user_sessions_user_id         ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_expires_at      ON user_sessions(expires_at);

-- --- Catalog FKs ---
CREATE INDEX idx_artists_label_id        ON artists(label_id);
CREATE INDEX idx_albums_label_id         ON albums(label_id);
CREATE INDEX idx_tracks_album_id         ON tracks(album_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_playlists_owner_id      ON playlists(owner_id);
CREATE INDEX idx_albums_genres_genre_id  ON albums_genres(genre_id);
CREATE INDEX idx_albums_release_date     ON albums(release_date DESC);

-- --- Junction reverse lookups ---
CREATE INDEX idx_albums_artists_artist_id        ON albums_artists(artist_id);
CREATE INDEX idx_tracks_artists_artist_id        ON tracks_artists(artist_id);
CREATE INDEX idx_users_playlists_playlist_id     ON users_playlists(playlist_id);
CREATE INDEX idx_playlists_tracks_track_id       ON playlists_tracks(track_id);
CREATE INDEX idx_playlists_tracks_order          ON playlists_tracks(playlist_id, track_order);
CREATE INDEX idx_user_liked_tracks_track_id      ON user_liked_tracks(track_id);
CREATE INDEX idx_user_saved_albums_album_id      ON user_saved_albums(album_id);
CREATE INDEX idx_user_followed_artists_artist_id ON user_followed_artists(artist_id);
CREATE INDEX idx_user_followed_playlists_pl_id   ON user_followed_playlists(playlist_id);

-- --- User library time-ordered lookups (newest first) ---
-- Backs "my liked tracks / saved albums / followed X, newest first".
CREATE INDEX idx_user_liked_tracks_user_liked_at
    ON user_liked_tracks(user_id, liked_at DESC);
CREATE INDEX idx_user_saved_albums_user_saved_at
    ON user_saved_albums(user_id, saved_at DESC);
CREATE INDEX idx_user_followed_artists_user_followed_at
    ON user_followed_artists(user_id, followed_at DESC);
CREATE INDEX idx_user_followed_playlists_user_followed_at
    ON user_followed_playlists(user_id, followed_at DESC);

-- --- Play history ---
CREATE INDEX idx_user_play_history_user_played_at ON user_play_history(user_id, played_at DESC);
CREATE INDEX idx_user_play_history_track_id       ON user_play_history(track_id);
CREATE INDEX idx_user_play_history_played_at      ON user_play_history(played_at DESC);

-- --- Whole-album plays ---
CREATE INDEX idx_user_album_plays_user_played_at
    ON user_album_plays(user_id, played_at DESC);
CREATE INDEX idx_user_album_plays_album_id
    ON user_album_plays(album_id);

-- --- Prefix search (case-insensitive via LOWER(...)) ---
-- Backs autocomplete with `LOWER(name) LIKE LOWER($1) || '%'`.
CREATE INDEX idx_tracks_name_prefix
    ON tracks (LOWER(name) text_pattern_ops) WHERE deleted_at IS NULL;
CREATE INDEX idx_artists_name_prefix
    ON artists (LOWER(name) text_pattern_ops);
CREATE INDEX idx_albums_name_prefix
    ON albums (LOWER(name) text_pattern_ops);

-- --- Trigram indexes for fuzzy/typo-tolerant search ---
CREATE INDEX idx_tracks_name_trgm
    ON tracks (name gin_trgm_ops) WHERE deleted_at IS NULL;
CREATE INDEX idx_artists_name_trgm
    ON artists (name gin_trgm_ops);
CREATE INDEX idx_albums_name_trgm
    ON albums (name gin_trgm_ops);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
-- 1. Drop explicitly named indexes.
DROP INDEX IF EXISTS idx_albums_name_trgm;
DROP INDEX IF EXISTS idx_artists_name_trgm;
DROP INDEX IF EXISTS idx_tracks_name_trgm;
DROP INDEX IF EXISTS idx_albums_name_prefix;
DROP INDEX IF EXISTS idx_artists_name_prefix;
DROP INDEX IF EXISTS idx_tracks_name_prefix;
DROP INDEX IF EXISTS idx_user_album_plays_album_id;
DROP INDEX IF EXISTS idx_user_album_plays_user_played_at;
DROP INDEX IF EXISTS idx_user_play_history_played_at;
DROP INDEX IF EXISTS idx_user_play_history_track_id;
DROP INDEX IF EXISTS idx_user_play_history_user_played_at;
DROP INDEX IF EXISTS idx_user_followed_playlists_user_followed_at;
DROP INDEX IF EXISTS idx_user_followed_artists_user_followed_at;
DROP INDEX IF EXISTS idx_user_saved_albums_user_saved_at;
DROP INDEX IF EXISTS idx_user_liked_tracks_user_liked_at;
DROP INDEX IF EXISTS idx_user_followed_playlists_pl_id;
DROP INDEX IF EXISTS idx_user_followed_artists_artist_id;
DROP INDEX IF EXISTS idx_user_saved_albums_album_id;
DROP INDEX IF EXISTS idx_user_liked_tracks_track_id;
DROP INDEX IF EXISTS idx_playlists_tracks_order;
DROP INDEX IF EXISTS idx_playlists_tracks_track_id;
DROP INDEX IF EXISTS idx_users_playlists_playlist_id;
DROP INDEX IF EXISTS idx_tracks_artists_artist_id;
DROP INDEX IF EXISTS idx_albums_artists_artist_id;
DROP INDEX IF EXISTS idx_albums_release_date;
DROP INDEX IF EXISTS idx_albums_genres_genre_id;
DROP INDEX IF EXISTS idx_playlists_owner_id;
DROP INDEX IF EXISTS idx_tracks_album_id;
DROP INDEX IF EXISTS idx_albums_label_id;
DROP INDEX IF EXISTS idx_artists_label_id;
DROP INDEX IF EXISTS idx_user_sessions_expires_at;
DROP INDEX IF EXISTS idx_user_sessions_user_id;
DROP INDEX IF EXISTS idx_password_reset_tokens_user_id;

-- 2. Drop dependent/junction tables.
DROP TABLE IF EXISTS track_audio_features;
DROP TABLE IF EXISTS user_album_plays;
DROP TABLE IF EXISTS user_play_history;
DROP TABLE IF EXISTS user_followed_playlists;
DROP TABLE IF EXISTS user_followed_artists;
DROP TABLE IF EXISTS user_saved_albums;
DROP TABLE IF EXISTS user_liked_tracks;
DROP TABLE IF EXISTS tracks_artists;
DROP TABLE IF EXISTS playlists_tracks;
DROP TABLE IF EXISTS users_playlists;
DROP TABLE IF EXISTS albums_artists;

-- 3. Drop core tables.
DROP TABLE IF EXISTS playlists;
DROP TABLE IF EXISTS tracks;
DROP TABLE IF EXISTS albums_genres;
DROP TABLE IF EXISTS genres;
DROP TABLE IF EXISTS albums;
DROP TABLE IF EXISTS artists;
DROP TABLE IF EXISTS record_labels;

-- 4. Drop base-level tables.
DROP TABLE IF EXISTS users_subscriptions;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS users;

-- 5. Drop functions, types, extensions.
DROP FUNCTION IF EXISTS ensure_album_has_artist();
DROP FUNCTION IF EXISTS create_free_subscription_for_user();
DROP FUNCTION IF EXISTS set_updated_at();
DROP TYPE IF EXISTS album_type;
DROP TYPE IF EXISTS user_tier;
DROP TYPE IF EXISTS artist_track_role;
DROP EXTENSION IF EXISTS pg_trgm;
DROP EXTENSION IF EXISTS citext;
-- +goose StatementEnd