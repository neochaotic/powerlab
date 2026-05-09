package service

import (
	"context"
	"crypto/ecdsa"
	"io"
	"mime/multipart"
	"os"

	"github.com/IceWhaleTech/CasaOS-Common/utils/jwt"
	"github.com/IceWhaleTech/CasaOS-UserService/service/model"
	"gorm.io/gorm"
)

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
// gorm.DB. It generates a fresh in-memory JWT signing keypair on
// every call — the private key is intentionally NEVER persisted, so
// every restart of the service issues tokens under a new key and
// invalidates outstanding sessions. This is a deliberate trade-off:
// session continuity across restarts is sacrificed for a stronger
// guarantee that a stolen disk image cannot forge tokens.
//
// Returns nil if keypair generation fails. Callers MUST check the
// return value before dereferencing.
func NewUserService(db *gorm.DB) UserService {
	privateKey, publicKey, err := jwt.GenerateKeyPair()
	if err != nil {
		_log.Error(context.Background(), "failed to generate key pair for JWT", err)
		return nil
	}

	return &userService{
		privateKey: privateKey,
		publicKey:  publicKey,
		db:         db,
	}
}
