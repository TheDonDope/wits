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

	dsn := os.Getenv("DATA_SOURCE_NAME")
	slog.Info("📁 🖥️  Using database", "dsn", dsn)

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	// Migrate the schema
	db.AutoMigrate(&types.User{})

	userStorage := &storage.UserStorage{DB: db}

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// HTTP Error Handler
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		slog.Error("🚨 🖥️  HTTP Error", "error", err, "path", c.Request().URL.Path)
	}

	// Serve public assets
	e.Static("/public", "public")

	// Index Route, redirect to login if necessary
	homeHandler := handler.HomeHandler{}
	e.GET("/", homeHandler.HandleHomeIndex)

	// Login routes
	loginHandler := handler.LoginHandler{UserStorage: userStorage}
	e.GET("/login", loginHandler.HandleGetLogin)
	e.POST("/login", loginHandler.HandlePostLogin)

	// Register routes
	registerHandler := handler.RegisterHandler{UserStorage: userStorage}
	e.GET("/register", registerHandler.HandleGetRegister)
	e.POST("/register", registerHandler.HandlePostRegister)

	// Dashboard routes
	r := e.Group("/dashboard")
	// Configure middleware with the custom claims type
	r.Use(echojwt.WithConfig(handler.EchoJWTConfig()))
	dashboardHandler := handler.DashboardHandler{}
	r.GET("", dashboardHandler.HandleGetDashboard)

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
