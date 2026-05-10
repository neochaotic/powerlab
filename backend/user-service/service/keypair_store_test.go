package service

import (
	"context"
	"crypto/elliptic"
	"errors"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// Tests for the JWT keypair persistence layer (issue #176, ADR-0020).
//
// The behavior these lock in is the PowerLab user-visible promise:
// restarts of user-service do not invalidate JWT cookies anymore,
// unless the operator opts into ephemeral mode via env var.

func freshDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}
	// Mirror the schema from migrations/0002_jwt_keypair.sql. The
	// real production path runs goose; tests use raw DDL because
	// goose needs a file-on-disk migrations FS.
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS jwt_keypair (
			id              INTEGER PRIMARY KEY CHECK (id = 1),
			private_key_pem TEXT NOT NULL,
			created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("create jwt_keypair table: %v", err)
	}
	return db
}

// TestLoadKeypair_NotFoundOnFreshDB locks the contract that an empty
// DB returns ErrKeypairNotFound (so callers know to generate + persist),
// rather than a generic SQL error that callers might mishandle.
func TestLoadKeypair_NotFoundOnFreshDB(t *testing.T) {
	db := freshDB(t)
	_, _, err := loadKeypair(db)
	if !errors.Is(err, ErrKeypairNotFound) {
		t.Fatalf("expected ErrKeypairNotFound on empty DB, got: %v", err)
	}
}

// TestSaveAndLoad_RoundTrip locks the persistence round-trip — the
// keypair the service generates today must be readable verbatim by
// the same service after restart.
func TestSaveAndLoad_RoundTrip(t *testing.T) {
	db := freshDB(t)
	priv, _, err := loadOrGenerateKeypair(context.Background(), db)
	if err != nil {
		t.Fatalf("first call (generate + persist): %v", err)
	}

	priv2, _, err := loadKeypair(db)
	if err != nil {
		t.Fatalf("second call (load): %v", err)
	}

	if priv.D.Cmp(priv2.D) != 0 {
		t.Errorf("private key D mismatch — round-trip lost integrity")
	}
	if priv.X.Cmp(priv2.X) != 0 || priv.Y.Cmp(priv2.Y) != 0 {
		t.Errorf("public key X/Y mismatch — round-trip lost integrity")
	}
	if priv.Curve != elliptic.P256() {
		t.Errorf("curve drifted: want P-256, got %v", priv.Curve.Params().Name)
	}
}

// TestLoadOrGenerate_StableAcrossCalls is THE regression test for
// #176. Two consecutive NewUserService-equivalent calls on the same
// DB must return the same keypair. If this fails, every restart of
// user-service will once again invalidate every JWT cookie.
func TestLoadOrGenerate_StableAcrossCalls(t *testing.T) {
	db := freshDB(t)
	ctx := context.Background()

	priv1, _, err := loadOrGenerateKeypair(ctx, db)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	priv2, _, err := loadOrGenerateKeypair(ctx, db)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if priv1.D.Cmp(priv2.D) != 0 {
		t.Fatal("keypair changed between consecutive calls — sessions would die on every restart (the v0.5.6 #176 bug)")
	}
}

// TestLoadOrGenerate_EphemeralMode_GeneratesFreshEachCall locks the
// opt-in escape hatch: when POWERLAB_EPHEMERAL_JWT_KEY=true, the old
// inherited behavior returns. Two consecutive calls produce DIFFERENT
// keypairs and the DB stays empty.
func TestLoadOrGenerate_EphemeralMode_GeneratesFreshEachCall(t *testing.T) {
	t.Setenv(EnvEphemeralJWTKey, "true")
	db := freshDB(t)
	ctx := context.Background()

	priv1, _, err := loadOrGenerateKeypair(ctx, db)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	priv2, _, err := loadOrGenerateKeypair(ctx, db)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if priv1.D.Cmp(priv2.D) == 0 {
		t.Error("ephemeral mode returned the same keypair across calls — the opt-out is broken")
	}

	// And the DB must remain empty in ephemeral mode.
	_, _, err = loadKeypair(db)
	if !errors.Is(err, ErrKeypairNotFound) {
		t.Errorf("ephemeral mode persisted to DB anyway — should have stayed in memory only")
	}
}

// TestLoadOrGenerate_PersistFailureIsNotFatal locks the behavior
// that a write failure (disk full, locked DB, etc.) does NOT block
// service startup. The service runs with the in-memory key, the
// next restart retries the persist. This avoids brick-on-disk-full.
func TestLoadOrGenerate_PersistFailureIsNotFatal(t *testing.T) {
	// Open a DB with no jwt_keypair table — saveKeypair will fail
	// with "no such table". Service should still get back a usable
	// keypair (degraded: not persisted).
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	priv, pub, err := loadOrGenerateKeypair(context.Background(), db)
	if err != nil {
		t.Fatalf("expected degraded-but-OK behavior, got error: %v", err)
	}
	if priv == nil || pub == nil {
		t.Fatal("expected non-nil keypair even on persist failure")
	}
}
