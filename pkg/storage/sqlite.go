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
	dsn := os.Getenv("SQLITE_DATA_SOURCE_NAME")
	slog.Info("📁 🏠 Using local sqlite database with", "dsn", dsn)
	var err error
	SQLiteDB, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		slog.Error("🚨 🏠 Opening local sqlite database failed with", "error", err)
		log.Fatal(err)
	}

	// Migrate the schema
	if automigrate {
		return SQLiteDB.AutoMigrate(&types.User{})
	}
	return nil
}
