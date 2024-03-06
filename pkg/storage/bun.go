package storage

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	"github.com/uptrace/bun"
	"github.com/uptrace/bun/dialect/pgdialect"
	"github.com/uptrace/bun/extra/bundebug"

	_ "github.com/lib/pq" // Importing the postgres driver
)

const (
	// DBTypeLocal is the variant of using a local postgresql database
	DBTypeLocal = "local"
	// DBTypeRemote is the variant of using a remote postgresql database
	DBTypeRemote = "remote"
)

// BunDB is the global database connection
var BunDB *bun.DB

// CreatePostgresDB creates a new database connection
func CreatePostgresDB(dbname string, dbuser string, dbpassword string, dbhost string) (*sql.DB, error) {
	slog.Info("💬 💾 (pkg/storage/bun.go) CreatePostgresDB()")
	hostArr := strings.Split(dbhost, ":")
	host := hostArr[0]
	port := "5432"
	if len(hostArr) > 1 {
		port = hostArr[1]
	}
	uri := fmt.Sprintf("user=%s password=%s dbname=%s host=%s port=%s sslmode=disable", dbuser, dbpassword, dbname, host, port)
	db, err := sql.Open("postgres", uri)
	if err != nil {
		slog.Info("🚨 💾 (pkg/storage/bun.go) ❓❓❓❓ 📂 Failed to create Postgresql db connection with", "error", err)
		return nil, err
	}
	slog.Info("✅ 💾 (pkg/storage/bun.go) CreatePostgresDB() -> 📂 Successfully created Postgresql db connection with", "host", dbhost)
	return db, nil
}

// InitBunWithPostgres initializes the bun database connection
func InitBunWithPostgres() error {
	slog.Info("💬 💾 (pkg/storage/bun.go) InitBunWithPostgres()")
	var (
		host   = os.Getenv("DB_HOST")
		user   = os.Getenv("DB_USER")
		pass   = os.Getenv("DB_PASSWORD")
		dbname = os.Getenv("DB_NAME")
	)
	db, err := CreatePostgresDB(dbname, user, pass, host)
	if err != nil {
		log.Fatal(err)
	}
	if err := db.Ping(); err != nil {
		slog.Info("🚨 💾 (pkg/storage/bun.go) ❓❓❓❓ 📂 Failed to ping Postgresql db with", "error", err)
		return err
	}
	BunDB = bun.NewDB(db, pgdialect.New())
	BunDB.AddQueryHook(bundebug.NewQueryHook(bundebug.WithVerbose(true)))
	slog.Info("✅ 💾 (pkg/storage/bun.go) InitBunWithPostgres() -> 📂 Successfully initialized Bun with Postgres db")
	return nil
}
