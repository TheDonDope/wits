// Package main is the entry point for the Wits server.
package main

import (
	"log"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/handler"
	"github.com/TheDonDope/wits/pkg/storage"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"github.com/joho/godotenv"
)

func main() {
	slog.Info("🥦 🖥️  Welcome to Wits!")

	if err := initEverything(); err != nil {
		log.Fatal(err)
	}

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
	a := handler.NewAuthHandler()
	e.GET("/login", a.HandleGetLogin)
	e.POST("/login", a.HandlePostLogin)
	e.POST("/logout", a.HandlePostLogout)
	e.GET("/register", a.HandleGetRegister)
	e.POST("/register", a.HandlePostRegister)
	e.GET("/auth/callback", a.HandleGetAuthCallback)

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

// initEverything initializes everything needed for the server to run
func initEverything() error {
	if err := godotenv.Load(); err != nil {
		return err
	}
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return storage.InitSQLiteDB(true)
	} else if dbType == storage.DBTypeRemote {
		return storage.InitSupabaseDB()
	}
	return nil
}
