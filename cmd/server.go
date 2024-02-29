// Package main is the entry point for the Wits server.
package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/handler"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/joho/godotenv"
)

func main() {
	slog.Info("🥦 🖥️  Welcome to Wits!")

	if err := initEverything(); err != nil {
		log.Fatal(err)
	}

	// Database
	dsn := os.Getenv("DATA_SOURCE_NAME")
	slog.Info("📁 🖥️  Using database", "dsn", dsn)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	// Migrate the schema
	db.AutoMigrate(&types.User{})

	// Storages
	u := &storage.UserStorage{DB: db}

	// Echo instance
	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Application wide HTTP Error Handler
	e.HTTPErrorHandler = handler.HTTPErrorHandler

	// Serve public assets
	e.Static("/public", "public")

	// Home Route
	h := handler.HomeHandler{}
	e.GET("/", h.HandleGetHome)

	// Auth routes
	a := handler.AuthHandler{Users: u}
	e.GET("/login", a.HandleGetLogin)
	e.POST("/login", a.HandlePostLogin)
	e.GET("/register", a.HandleGetRegister)
	e.POST("/register", a.HandlePostRegister)

	// Dashboard routes
	d := handler.DashboardHandler{}
	g := e.Group("/dashboard")
	// Configure middleware with the custom claims type
	g.Use(echojwt.WithConfig(handler.EchoJWTConfig()))
	g.GET("", d.HandleGetDashboard)

	// Start server
	addr := os.Getenv("HTTP_LISTEN_ADDR")
	slog.Info("🚀 🖥️  Wits server is running at", "addr", addr)
	e.Logger.Fatal(e.Start(addr))
}

func initEverything() error {
	// if err := godotenv.Load(); err != nil {
	// 	return err
	// }
	return godotenv.Load()
}
