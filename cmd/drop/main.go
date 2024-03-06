// Package main is the entry point for the Database drop operation
package main

import (
	"database/sql"
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/storage"

	"github.com/joho/godotenv"

	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	slog.Info("💬 💾 (cmd/drop/main.go) 🥦 Welcome to Wits Database Dropper!")
	db, err := createDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	tables := []string{
		"schmema_migrations",
		"accounts",
	}

	for _, table := range tables {
		query := fmt.Sprintf("drop table if exists %s cascade", table)
		if _, err := db.Exec(query); err != nil {
			log.Fatal(err)
		}
		slog.Info("🆗 💾 (cmd/drop/main.go)  🫳 Dropped", "table", table)
	}

	if os.Getenv("DB_TYPE") == storage.DBTypeLocal {
		if _, err := db.Exec("drop schema if exists auth cascade"); err != nil {
			log.Fatal(err)
		}
		slog.Info("🆗 💾 (cmd/drop/main.go)  🫳 Dropped local schema auth")
	}

	slog.Info("✅ 💾 (cmd/drop/main.go) 🥦 Wits Database Dropper finished!")
}

func createDB() (*sql.DB, error) {
	slog.Info("💬 💾 (cmd/drop/main.go) createDB()")
	if err := godotenv.Load(); err != nil {
		return nil, err
	}
	var (
		host   = os.Getenv("DB_HOST")
		user   = os.Getenv("DB_USER")
		pass   = os.Getenv("DB_PASSWORD")
		dbname = os.Getenv("DB_NAME")
	)

	return storage.CreatePostgresDB(dbname, user, pass, host)
}
