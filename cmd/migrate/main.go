// Package main is the entry point for the Database migrations
package main

import (
	"database/sql"
	"log"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	"github.com/joho/godotenv"

	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	slog.Info("💬 💾 (cmd/migrate/main.go) 🥦 Welcome to Wits Database Migrator!")
	db, err := createDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Create migration instance
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Point to your migration files. Here we're using local files, but it could be other sources.
	m, err := migrate.NewWithDatabaseInstance(
		"file://cmd/migrate/migrations", // source URL
		"postgres",                      // driver name
		driver,                          // instance
	)
	if err != nil {
		log.Fatal(err)
	}

	cmd := os.Args[len(os.Args)-1]
	if cmd == "up" {
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
		slog.Info("🆗 💾 (cmd/migrate/main.go)  💾 Up migrations ran!")
	}
	if cmd == "down" {
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			log.Fatal(err)
		}
		slog.Info("🆗 💾 (cmd/migrate/main.go)  💾 Down migrations ran!")
	}
	slog.Info("✅ 💾 (cmd/migrate/main.go) 🥦 Wits Database Migrator finished!")
}

func createDB() (*sql.DB, error) {
	slog.Info("💬 💾 (cmd/migrate/main.go) createDB()")
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
