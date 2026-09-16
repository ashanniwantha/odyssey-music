-- +goose Up
SELECT 'up SQL query';

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS users (
    id UUID,
    username TEXT NOT NULL,
    email citext NOT NULL,
    hashed_password TEXT NOT NULL,
    first_name TEXT,
    last_name TEXT,
    DOB DATE NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_users PRIMARY KEY (id),
    CONSTRAINT uq_users_username UNIQUE (username),
    CONSTRAINT uq_users_email UNIQUE (email),
    CONSTRAINT chk_users_username_not_empty CHECK (length(trim(username)) > 0),
    CONSTRAINT chk_users_email_not_empty CHECK (length(trim(email)) > 0)
);

CREATE TABLE IF NOT EXISTS artists (
    id UUID,
    name TEXT NOT NULL,
    description TEXT,
    country TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_artists PRIMARY KEY (id),
    CONSTRAINT chk_artists_name_not_empty CHECK (length(trim(name)) > 0)
);

CREATE TABLE IF NOT EXISTS albums (
    id UUID,
    name TEXT NOT NULL,
    genre TEXT NOT NULL,
    release_date DATE NOT NULL,
    label TEXT,
    cover_art TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_albums PRIMARY KEY (id),
    CONSTRAINT chk_albums_name_not_empty CHECK (length(trim(name)) > 0),
    CONSTRAINT chk_albums_genre_not_empty CHECK (length(trim(genre)) > 0),
    CONSTRAINT chk_albums_cover_art_not_empty CHECK (length(trim(cover_art)) > 0)
);

CREATE TABLE IF NOT EXISTS tracks (
    id UUID,
    name TEXT NOT NULL,
    album_id UUID NOT NULL,
    duration_seconds INT NOT NULL, -- Added: crucial for audio processing
    storage_path TEXT NOT NULL,     -- Added: track location on disk or S3
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_tracks PRIMARY KEY (id),
    CONSTRAINT fk_tracks_albums FOREIGN KEY (album_id) REFERENCES albums(id) ON DELETE RESTRICT,
    CONSTRAINT chk_tracks_name_not_empty CHECK (length(trim(name)) > 0),
    CONSTRAINT chk_tracks_storage_path_not_empty CHECK (length(trim(storage_path)) > 0),
    CONSTRAINT chk_tracks_duration_positive CHECK (duration_seconds > 0)
);

CREATE TABLE IF NOT EXISTS playlists (
    id UUID,
    name TEXT NOT NULL,
    cover_art TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT pk_playlists PRIMARY KEY (id),
    CONSTRAINT chk_playlists_name_not_empty CHECK (length(trim(name)) > 0)
);

CREATE TABLE IF NOT EXISTS albums_artists (
    album_id UUID NOT NULL,
    artist_id UUID NOT NULL,

    CONSTRAINT pk_albums_artists PRIMARY KEY (album_id, artist_id), 
    CONSTRAINT fk_albums_artists_albums FOREIGN KEY (album_id) REFERENCES albums(id) ON DELETE CASCADE,
    CONSTRAINT fk_albums_artists_artists FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS users_playlists (
    user_id UUID NOT NULL,
    playlist_id UUID NOT NULL,

    CONSTRAINT pk_users_playlists PRIMARY KEY (user_id, playlist_id), 
    CONSTRAINT fk_users_playlists_users FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_users_playlists_playlists FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS playlists_tracks (
    playlist_id UUID NOT NULL,
    track_id UUID NOT NULL,

    CONSTRAINT pk_playlists_tracks PRIMARY KEY (playlist_id, track_id),
    CONSTRAINT fk_playlists_tracks_playlists FOREIGN KEY (playlist_id) REFERENCES playlists(id) ON DELETE CASCADE,
    CONSTRAINT fk_playlists_tracks_tracks FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE
);

-- Create an ENUM or a text column with a check constraint for artist roles
CREATE TYPE artist_track_role AS ENUM ('main', 'feature', 'producer', 'remixer');

CREATE TABLE IF NOT EXISTS tracks_artists (
    track_id UUID NOT NULL,
    artist_id UUID NOT NULL,
    role artist_track_role NOT NULL DEFAULT 'main',

    CONSTRAINT pk_tracks_artists PRIMARY KEY (track_id, artist_id),
    CONSTRAINT fk_tracks_artists_tracks FOREIGN KEY (track_id) REFERENCES tracks(id) ON DELETE CASCADE,
    CONSTRAINT fk_tracks_artists_artists FOREIGN KEY (artist_id) REFERENCES artists(id) ON DELETE CASCADE
);

-- +goose Down
SELECT 'down SQL query';

-- 1. Drop junction/dependent tables first to clear foreign key relations
DROP TABLE IF EXISTS tracks_artists;
DROP TABLE IF EXISTS playlists_tracks;
DROP TABLE IF EXISTS users_playlists;
DROP TABLE IF EXISTS albums_artists;

-- 2. Drop core entity tables that depend on others (tracks depends on albums)
DROP TABLE IF EXISTS tracks;
DROP TABLE IF EXISTS playlists;
DROP TABLE IF EXISTS albums;

-- 3. Drop base level independent entities
DROP TABLE IF EXISTS artists;
DROP TABLE IF EXISTS users;

-- 4. Drop custom data types and extensions last
DROP TYPE IF EXISTS artist_track_role;
DROP EXTENSION IF EXISTS citext;
