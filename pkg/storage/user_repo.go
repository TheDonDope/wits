package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
	"golang.org/x/crypto/bcrypt"
)

// CreateAuthenticatedUser creates an authenticated user in the database
func CreateAuthenticatedUser(user *types.AuthenticatedUser) error {
	slog.Info("💬 💾 (pkg/storage/user_repo.go) CreateAuthenticatedUser()")
	_, err := BunDB.NewInsert().Model(user).Exec(context.Background())
	slog.Info("✅ 💾 (pkg/storage/user_repo.go) CreateAuthenticatedUser() -> 📂 Authenticated user creation finished with", "error", err)
	return err
}

// GetAuthenticatedUserByEmailAndPassword retrieves an authenticated user by the email and password
func GetAuthenticatedUserByEmailAndPassword(email string, password string) (types.AuthenticatedUser, error) {
	slog.Info("💬 💾 (pkg/storage/user_repo.go) GetAuthenticatedUserByEmailAndPassword()")
	user, err := GetAuthenticatedUserByEmail(email)

	if errors.Is(err, sql.ErrNoRows) {
		slog.Info("✅ 💾 (pkg/storage/user_repo.go) GetAuthenticatedUserByEmailAndPassword() -> 📖 No user with email found, returning empty user")
		return types.AuthenticatedUser{}, err
	}

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
	err := BunDB.NewSelect().Model(&user).Where("email = ?", email).Scan(context.Background())
	slog.Info("✅ 💾 (pkg/storage/user_repo.go) GetAuthenticatedUserByEmail() -> 📂 Authenticated user retrieval finished with", "user", user, "error", err)
	return user, err
}
