package service

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// EnvEphemeralJWTKey, when set to "true", switches user-service back
// to the pre-v0.5.7 behavior of generating a fresh in-memory keypair
// on every startup (and never persisting it). Default is empty/false,
// which means the keypair is loaded from / persisted to the user.db
// jwt_keypair table.
//
// Use case for opt-in: operators in higher-threat environments who
// would rather force every-restart re-login than risk a stolen disk
// image being able to forge tokens. See ADR-0020 for the trade-off.
const EnvEphemeralJWTKey = "POWERLAB_EPHEMERAL_JWT_KEY"

// ErrKeypairNotFound signals that no keypair has been persisted to
// the jwt_keypair table yet — caller should generate one and call
// saveKeypair.
var ErrKeypairNotFound = errors.New("no persisted JWT keypair")

// loadKeypair reads the PEM-encoded ECDSA private key from the
// single-row jwt_keypair table and returns it parsed. Public key is
// derived from the private key.
//
// Returns ErrKeypairNotFound when the row doesn't exist (fresh DB).
// Returns a wrapped error for any other failure (corrupt PEM, wrong
// key type, etc.) — caller should NOT silently regenerate in those
// cases; it would mask data corruption.
func loadKeypair(db *gorm.DB) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	var pemStr string
	err := db.Raw("SELECT private_key_pem FROM jwt_keypair WHERE id = 1").Row().Scan(&pemStr)
	if err != nil {
		// "no rows" → empty table → first boot, not an error.
		if errors.Is(err, gorm.ErrRecordNotFound) || err.Error() == "sql: no rows in result set" {
			return nil, nil, ErrKeypairNotFound
		}
		// "no such table" → migration hasn't run yet (or this is a
		// degraded test environment without the schema). Treat as
		// first-boot — caller will generate + try to persist. The
		// persist will fail too, but the service still starts with
		// an in-memory key.
		if strings.Contains(err.Error(), "no such table") {
			return nil, nil, ErrKeypairNotFound
		}
		return nil, nil, fmt.Errorf("read jwt_keypair: %w", err)
	}

	block, _ := pem.Decode([]byte(pemStr))
	if block == nil || block.Type != "EC PRIVATE KEY" {
		return nil, nil, fmt.Errorf("decode pem: not an EC PRIVATE KEY block")
	}

	priv, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("parse ec private key: %w", err)
	}

	return priv, &priv.PublicKey, nil
}

// saveKeypair persists the given ECDSA private key to the single-row
// jwt_keypair table. INSERT OR REPLACE ensures idempotent overwrites
// (e.g. if a future operator manually wipes the row to force rotation).
func saveKeypair(db *gorm.DB, priv *ecdsa.PrivateKey) error {
	der, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return fmt.Errorf("marshal ec private key: %w", err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: der,
	})
	if err := db.Exec(
		"INSERT OR REPLACE INTO jwt_keypair (id, private_key_pem) VALUES (1, ?)",
		string(pemBytes),
	).Error; err != nil {
		return fmt.Errorf("write jwt_keypair: %w", err)
	}
	return nil
}
