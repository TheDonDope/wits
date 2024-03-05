package storage

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// ReadByEmailAndPassword tries to retrieve an user with the given name and password from the local sqlite database.
// If no user is found, an empty user and the error are returned.
func ReadByEmailAndPassword(email string, password string) (types.User, error) {
	slog.Info("💬 💾 (pkg/storage/user.go) ReadByEmailAndPassword()")
	user, err := ReadByEmail(email)
	if err != nil {
		slog.Error("🚨 💾 (pkg/storage/user.go) ❓❓❓❓ 📖 Finding user by email failed with", "error", err)
		return types.User{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		slog.Error("🚨 💾 (pkg/storage/user.go) ❓❓❓❓ 📖 Password is incorrect")
		return types.User{}, fmt.Errorf("(pkg/storage/user.go) Password is incorrect")
	}

	slog.Info("✅ 💾 (pkg/storage/user.go) ReadByEmailAndPassword() -> Found user by email and password with", "email", user.Email, "name", user.Name)
	return user, nil
}

// ReadByEmail tries to retrieve an user with the given name from the local sqlite database.
// If no user is found, an empty user and the error are returned.
func ReadByEmail(email string) (types.User, error) {
	slog.Info("💬 💾 (pkg/storage/user.go) readByEmail()")
	var user types.User
	err := SQLiteDB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("🚨 💾 (pkg/storage/user.go) ❓❓❓❓ 📖 Finding user failed with", "error", err)
		return types.User{}, err
	}

	slog.Info("✅ 💾 (pkg/storage/user.go) readByEmail() -> Found user by email and password with", "email", user.Email, "name", user.Name)
	return user, nil
}
