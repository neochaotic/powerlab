package service

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/neochaotic/powerlab/backend/user-service/pkg/utils/encryption"
	"github.com/neochaotic/powerlab/backend/user-service/service/model"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) (*gorm.DB, string) {
	tempDir, err := os.MkdirTemp("", "user-service-test-*")
	assert.NoError(t, err)

	dbPath := filepath.Join(tempDir, "test_user.db")
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&model.UserDBModel{})
	assert.NoError(t, err)

	return db, tempDir
}

func TestUserManagementWithBcrypt(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	userService := NewUserService(db)
	assert.NotNil(t, userService)

	// 1. Create User
	password := "secure_password_123"
	hashedPassword, err := encryption.HashPassword(password)
	assert.NoError(t, err)
	assert.NotEqual(t, password, hashedPassword)

	user := model.UserDBModel{
		Username: "admin",
		Password: hashedPassword,
		Role:     "admin",
	}

	createdUser := userService.CreateUser(user)
	assert.NotZero(t, createdUser.Id)
	assert.Equal(t, "admin", createdUser.Username)

	// 2. Verify Login (CheckPasswordHash)
	foundUser := userService.GetUserAllInfoByName("admin")
	assert.Equal(t, createdUser.Id, foundUser.Id)
	
	isValid := encryption.CheckPasswordHash(password, foundUser.Password)
	assert.True(t, isValid, "Password should be valid with bcrypt")

	// 3. Verify Invalid Login
	isValid = encryption.CheckPasswordHash("wrong_password", foundUser.Password)
	assert.False(t, isValid, "Password should be invalid")

	// 4. Update Password
	newPassword := "new_secret_456"
	newHashed, _ := encryption.HashPassword(newPassword)
	foundUser.Password = newHashed
	userService.UpdateUserPassword(foundUser)

	updatedUser := userService.GetUserAllInfoByName("admin")
	assert.True(t, encryption.CheckPasswordHash(newPassword, updatedUser.Password))
}

func TestUserCount(t *testing.T) {
	db, tempDir := setupTestDB(t)
	defer os.RemoveAll(tempDir)

	userService := NewUserService(db)
	
	count := userService.GetUserCount()
	assert.Equal(t, int64(0), count)

	userService.CreateUser(model.UserDBModel{Username: "user1", Password: "p1"})
	userService.CreateUser(model.UserDBModel{Username: "user2", Password: "p2"})

	count = userService.GetUserCount()
	assert.Equal(t, int64(2), count)
}
