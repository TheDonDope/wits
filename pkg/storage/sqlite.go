package storage

import (
	"log"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// DBTypeLocal is the variant of using a local sqlite database
const DBTypeLocal = "local"

// SQLiteDB is the SQLite database for the application
var SQLiteDB *gorm.DB

// InitSQLiteDB initializes the SQLite database.
func InitSQLiteDB(automigrate bool) error {
	slog.Info("💬 🏠 (pkg/storage/sqlite.go) InitSQLiteDB()")
	dsn := os.Getenv("SQLITE_DATA_SOURCE_NAME")
	slog.Info("🆗 🏠 (pkg/storage/sqlite.go)  📂 Using", "dsn", dsn)
	var err error
	SQLiteDB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		slog.Error("🚨 🏠 (pkg/storage/sqlite.go) ❓❓❓❓ 📂 Opening local sqlite database failed with", "error", err)
		log.Fatal(err)
	}

	// Migrate the schema
	if automigrate {
		slog.Info("✅ 🏠 (pkg/storage/sqlite.go) InitSQLiteDB() -> 📂 Initialized sqlite db with automigrations")
		return SQLiteDB.AutoMigrate(&types.User{})
	}
	slog.Info("✅ 🏠 (pkg/storage/sqlite.go) InitSQLiteDB() -> 📂 Initialized sqlite db without automigrations")
	return nil
}
