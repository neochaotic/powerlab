-- +goose Up
-- Initial user-service schema. Captures what GORM's AutoMigrate
-- (on UserDBModel + EventModel) produced before the migration runner
-- was wired in (Sprint 3 Phase 2 — issue #100).
--
-- IF NOT EXISTS is critical: existing installs (pre-v0.6.0) already
-- have these tables created by AutoMigrate. Running this migration
-- on those installs must be a no-op so they end up at goose v1
-- without errors.

CREATE TABLE IF NOT EXISTS o_users (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    username    TEXT,
    password    TEXT,
    role        TEXT,
    email       TEXT,
    nickname    TEXT,
    avatar      TEXT,
    description TEXT,
    source      TEXT,
    uid         TEXT,
    created_at  DATETIME,
    updated_at  DATETIME
);

CREATE TABLE IF NOT EXISTS events (
    uuid       TEXT PRIMARY KEY,
    source_id  TEXT,
    name       TEXT,
    properties TEXT,
    timestamp  INTEGER
);

CREATE INDEX IF NOT EXISTS idx_events_source_id ON events (source_id);

-- +goose Down
DROP INDEX IF EXISTS idx_events_source_id;
DROP TABLE IF EXISTS events;
DROP TABLE IF EXISTS o_users;
