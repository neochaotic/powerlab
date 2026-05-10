-- +goose Up
-- JWT signing keypair persistence (issue #176).
--
-- Pre-v0.5.7 user-service generated a fresh in-memory keypair on every
-- restart (NewUserService) and never persisted it. Result: every
-- service restart — including every in-app upgrade — invalidated all
-- outstanding JWT cookies, silently logging out every user.
--
-- The pre-v0.5.7 godoc described this as a "deliberate trade-off"
-- against stolen-disk-image attackers forging tokens. Per ADR-0020:
-- the threat model doesn't justify the UX cost for a self-hosted home
-- server. We persist by default. Operators in higher-threat
-- environments can opt out via POWERLAB_EPHEMERAL_JWT_KEY=true env var.
--
-- Schema: a single-row table. Multi-row support is not needed (one
-- keypair per service instance). The CHECK constraint on id ensures
-- INSERT ... OR REPLACE always targets the same row.
CREATE TABLE IF NOT EXISTS jwt_keypair (
    id              INTEGER PRIMARY KEY CHECK (id = 1),
    private_key_pem TEXT NOT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- +goose Down
DROP TABLE IF EXISTS jwt_keypair;
