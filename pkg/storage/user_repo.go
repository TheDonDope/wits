package storage

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// CreateAuthenticatedUser creates an authenticated user in the database
func CreateAuthenticatedUser(user *types.AuthenticatedUser) error {
	slog.Info("💬 💾 (pkg/storage/user_repo.go) CreateAuthenticatedUser()")
	_, err := BunDB.NewInsert().Model(&user).Exec(context.Background())
	slog.Info("✅ 💾 (pkg/storage/user_repo.go) CreateAuthenticatedUser() -> 📂 Authenticated user creation finished with", "error", err)
	return err
}

// GetAuthenticatedUserByEmailAndPassword retrieves an authenticated user by the email and password
func GetAuthenticatedUserByEmailAndPassword(email string, password string) (types.AuthenticatedUser, error) {
	slog.Info("💬 💾 (pkg/storage/user_repo.go) GetAuthenticatedUserByEmailAndPassword()")
	user, err := GetAuthenticatedUserByEmail(email)
	if err != nil {
		slog.Error("🚨 💾 (pkg/storage/user_repo.go) ❓❓❓❓ 📖 Finding user by email failed with", "error", err)
		return types.AuthenticatedUser{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		slog.Error("🚨 💾 (pkg/storage/user_repo.go) ❓❓❓❓ 📖 Password is incorrect")
		return types.AuthenticatedUser{}, fmt.Errorf("(pkg/storage/user_repo.go) Password is incorrect")
	}
	slog.Info("✅ 💾 (pkg/storage/user_repo.go) GetAuthenticatedUserByEmailAndPassword() -> Found user by email and password with", "email", user.Email)
	return user, err

}

// GetAuthenticatedUserByEmail retrieves an authenticated user by the email
func GetAuthenticatedUserByEmail(email string) (types.AuthenticatedUser, error) {
	slog.Info("💬 💾 (pkg/storage/user_repo.go) GetAuthenticatedUserByEmail()")
	var user types.AuthenticatedUser
	err := BunDB.NewSelect().Model(&user).Relation("Account").Where("email = ?", email).Scan(context.Background())
	slog.Info("✅ 💾 (pkg/storage/user_repo.go) GetAuthenticatedUserByEmail() -> 📂 Authenticated user retrieval finished with", "user", user, "error", err)
	return user, err
}

// ReadByEmailAndPassword tries to retrieve an user with the given name and password from the local sqlite database.
// If no user is found, an empty user and the error are returned.
func ReadByEmailAndPassword(email string, password string) (types.User, error) {
	slog.Info("💬 💾 (pkg/storage/user_repo.go) ReadByEmailAndPassword()")
	user, err := ReadByEmail(email)
	if err != nil {
		slog.Error("🚨 💾 (pkg/storage/user_repo.go) ❓❓❓❓ 📖 Finding user by email failed with", "error", err)
		return types.User{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		slog.Error("🚨 💾 (pkg/storage/user_repo.go) ❓❓❓❓ 📖 Password is incorrect")
		return types.User{}, fmt.Errorf("(pkg/storage/user_repo.go) Password is incorrect")
	}

	slog.Info("✅ 💾 (pkg/storage/user_repo.go) ReadByEmailAndPassword() -> Found user by email and password with", "email", user.Email, "name", user.Name)
	return user, nil
}

// ReadByEmail tries to retrieve an user with the given name from the local sqlite database.
// If no user is found, an empty user and the error are returned.
func ReadByEmail(email string) (types.User, error) {
	slog.Info("💬 💾 (pkg/storage/user_repo.go) readByEmail()")
	var user types.User
	err := SQLiteDB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("🚨 💾 (pkg/storage/user_repo.go) ❓❓❓❓ 📖 Finding user failed with", "error", err)
		return types.User{}, err
	}

	slog.Info("✅ 💾 (pkg/storage/user_repo.go) readByEmail() -> Found user by email", "email", user.Email, "name", user.Name)
	return user, nil
}
