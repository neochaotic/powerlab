package service

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"os"

	"github.com/neochaotic/powerlab/backend/common/utils/jwt"
	"github.com/neochaotic/powerlab/backend/user-service/service/model"
	"gorm.io/gorm"
)

// UserService is the data-access + auth surface backing the
// user-service routes. CRUD on the `o_users` table plus the JWT
// signing keypair (loaded from / persisted to user.db per
// ADR-0020 so sessions survive service restarts).
type UserService interface {
	UpLoadFile(file multipart.File, name string) error
	CreateUser(m model.UserDBModel) model.UserDBModel
	GetUserCount() (userCount int64)
	UpdateUser(m model.UserDBModel)
	UpdateUserPassword(m model.UserDBModel)
	GetUserInfoById(id string) (m model.UserDBModel)
	GetUserAllInfoById(id string) (m model.UserDBModel)
	GetUserAllInfoByName(userName string) (m model.UserDBModel)
	DeleteUserById(id string)
	DeleteAllUser()
	GetUserInfoByUserName(userName string) (m model.UserDBModel)
	GetAllUserName() (list []model.UserDBModel)

	GetKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey)
}

var UserRegisterHash = make(map[string]string)

type userService struct {
	privateKey *ecdsa.PrivateKey // keep this private - NEVER expose it!!!
	publicKey  *ecdsa.PublicKey

	db *gorm.DB
}

func (u *userService) DeleteAllUser() {
	u.db.Where("1=1").Delete(&model.UserDBModel{})
}

func (u *userService) DeleteUserById(id string) {
	u.db.Where("id= ?", id).Delete(&model.UserDBModel{})
}

func (u *userService) GetAllUserName() (list []model.UserDBModel) {
	u.db.Select("username").Find(&list)
	return
}

func (u *userService) CreateUser(m model.UserDBModel) model.UserDBModel {
	u.db.Create(&m)
	return m
}

func (u *userService) GetUserCount() (userCount int64) {
	u.db.Find(&model.UserDBModel{}).Count(&userCount)
	return
}

func (u *userService) UpdateUser(m model.UserDBModel) {
	u.db.Model(&m).Omit("password").Updates(&m)
}

func (u *userService) UpdateUserPassword(m model.UserDBModel) {
	u.db.Model(&m).Update("password", m.Password)
}

func (u *userService) GetUserAllInfoById(id string) (m model.UserDBModel) {
	u.db.Where("id= ?", id).First(&m)
	return
}

func (u *userService) GetUserAllInfoByName(userName string) (m model.UserDBModel) {
	u.db.Where("username= ?", userName).First(&m)
	return
}

func (u *userService) GetUserInfoById(id string) (m model.UserDBModel) {
	u.db.Select("username", "id", "role", "nickname", "description", "avatar", "email").Where("id= ?", id).First(&m)
	return
}

func (u *userService) GetUserInfoByUserName(userName string) (m model.UserDBModel) {
	u.db.Select("username", "id", "role", "nickname", "description", "avatar", "email").Where("username= ?", userName).First(&m)
	return
}

// 上传文件
func (c *userService) UpLoadFile(file multipart.File, url string) error {
	out, _ := os.OpenFile(url, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0o644)
	defer out.Close()
	io.Copy(out, file)
	return nil
}

func (u *userService) GetKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	return u.privateKey, u.publicKey
}

// NewUserService constructs a UserService backed by the supplied
// gorm.DB. The JWT signing keypair is loaded from the user.db
// `jwt_keypair` table; on first boot (or after a manual wipe of the
// row) a fresh keypair is generated AND persisted. Restart, in-app
// upgrade, and crash recovery all preserve the keypair → existing
// JWT cookies stay valid across service lifecycles.
//
// Pre-v0.5.7 behavior was to generate a fresh in-memory keypair on
// every call and never persist it (a comment described this as a
// "deliberate trade-off" against stolen-disk-image attackers
// forging tokens). Per ADR-0020 we revisited that — the threat model
// doesn't justify the UX cost for a self-hosted home server, where
// disk-physical-access already implies bigger problems than JWT
// forge.
//
// Operators in higher-threat environments can opt back into the
// ephemeral behavior by setting POWERLAB_EPHEMERAL_JWT_KEY=true
// (see EnvEphemeralJWTKey constant in keypair_store.go).
//
// Returns nil if keypair load/generate fails. Callers MUST check the
// return value before dereferencing.
func NewUserService(db *gorm.DB) UserService {
	ctx := context.Background()
	privateKey, publicKey, err := loadOrGenerateKeypair(ctx, db)
	if err != nil {
		_log.Error(ctx, "failed to load/generate key pair for JWT", err)
		return nil
	}

	return &userService{
		privateKey: privateKey,
		publicKey:  publicKey,
		db:         db,
	}
}

// loadOrGenerateKeypair encapsulates the env-var + DB-load + generate
// fallback flow so it can be unit-tested without spinning up a full
// userService. Behavior:
//
//  1. POWERLAB_EPHEMERAL_JWT_KEY=true → always generate fresh, never
//     persist. Pre-v0.5.7 behavior preserved as opt-in.
//  2. Else load from DB. If found, return.
//  3. Else (first boot / wiped row) generate fresh, persist, return.
//     A persist failure is downgraded to a warning — we still return
//     the generated keypair so the service starts; the next restart
//     will retry the persist. This avoids brick-on-disk-full.
func loadOrGenerateKeypair(ctx context.Context, db *gorm.DB) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	if os.Getenv(EnvEphemeralJWTKey) == "true" {
		_log.Info(ctx, "POWERLAB_EPHEMERAL_JWT_KEY=true — keypair will not be persisted (sessions reset every restart)")
		return jwt.GenerateKeyPair()
	}

	priv, pub, err := loadKeypair(db)
	if err == nil {
		return priv, pub, nil
	}
	if !errors.Is(err, ErrKeypairNotFound) {
		// Corrupt persisted row — bail rather than silently overwrite
		// what might be the real key with a generated one (which would
		// invalidate every existing JWT and mask the underlying error).
		return nil, nil, fmt.Errorf("load persisted keypair: %w", err)
	}

	priv, pub, err = jwt.GenerateKeyPair()
	if err != nil {
		return nil, nil, fmt.Errorf("generate fresh keypair: %w", err)
	}
	if err := saveKeypair(db, priv); err != nil {
		// Don't fail startup over persist failure (disk full, locked
		// DB, etc.) — service still works with the in-memory key, the
		// next restart retries the persist.
		_log.Warn(ctx, "generated fresh JWT keypair but failed to persist — sessions will reset on next restart",
			slog.String("error", err.Error()))
	}
	return priv, pub, nil
}
