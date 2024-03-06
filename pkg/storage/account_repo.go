package storage

import (
	"context"
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/google/uuid"
)

// GetAccountByUserID retrieves an account by the user ID
func GetAccountByUserID(userID uuid.UUID) (types.Account, error) {
	slog.Info("💬 🛰️  (pkg/storage/account_repo.go) GetAccountByUserID()")
	var account types.Account
	err := BunDB.NewSelect().Model(&account).Where("user_id = ?", userID).Scan(context.Background())
	slog.Info("✅ 🛰️  (pkg/storage/account_repo.go) GetAccountByUserID() -> 📂 Account retrieval finished with", "error", err)
	return account, err
}

// CreateAccount creates an account in the database
func CreateAccount(account *types.Account) error {
	slog.Info("💬 🛰️  (pkg/storage/account_repo.go) CreateAccount()")
	_, err := BunDB.NewInsert().Model(account).Exec(context.Background())
	slog.Info("✅ 🛰️  (pkg/storage/account_repo.go) CreateAccount() -> 📂 Account creation finished with", "error", err)
	return err
}
