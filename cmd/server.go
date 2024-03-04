// Package main is the entry point for the Wits server.
package main

import (
	"io"
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
	slog.Info("💬 🖥️  (cmd/server.go) 🥦 Welcome to Wits!")

	if err := initEverything(); err != nil {
		log.Fatal(err)
	}

	// Echo instance
	e := echo.New()

	if err := configureLogging(e); err != nil {
		log.Fatal(err)
	}

	// Application wide HTTP Error Handler
	e.HTTPErrorHandler = handler.HTTPErrorHandler

	// Serve public assets
	e.Static("/public", "public")

	// Home Route
	h := handler.HomeHandler{}
	e.GET("/", h.HandleGetHome)

	// Auth routes
	a := handler.NewAuthHandler()
	e.Use(handler.WithUser())
	e.GET("/login", a.HandleGetLogin)
	e.GET("/login/provider/google", a.HandleGetLoginWithGoogle)
	e.POST("/login", a.HandlePostLogin)
	e.POST("/logout", a.HandlePostLogout)
	e.GET("/register", a.HandleGetRegister)
	e.POST("/register", a.HandlePostRegister)
	e.GET("/auth/callback", a.HandleGetAuthCallback)

	// Authenticated routes
	g := e.Group("") // Start with root path
	// Configure middleware with the custom claims type, but only when using local DB
	if os.Getenv("DB_TYPE") == storage.DBTypeLocal {
		g.Use(echojwt.WithConfig(handler.EchoJWTConfig()))
	}

	g.Use(handler.WithAuth())

	// Dashboard routes
	d := handler.DashboardHandler{}
	g.GET("/dashboard", d.HandleGetDashboard)

	// User settings routes
	s := handler.SettingsHandler{}
	g.GET("/settings", s.HandleGetSettings)

	// Start server
	addr := os.Getenv("HTTP_LISTEN_ADDR")
	slog.Info("🚀 🖥️  (cmd/server.go) 🛜 Wits server is running at", "addr", addr)
	e.Logger.Fatal(e.Start(addr))
}

// configureLogging configures the logging for the server, adding Logging and Recovery middlewares as well as
// setting the log level from the environment. Finally, it sets the log output to a stdout and file.
//
// IMPORTANT: If the 'log' folder does not exist, the server will panic. This behaviour might be subject to further
// change. (We might want to create the folder if it does not exist, for example.)
func configureLogging(e *echo.Echo) error {
	slog.Info("💬 🖥️  (cmd/server.go) configureLogging()")

	// Set log level from environment variable
	e.Logger.SetLevel(handler.ParseLogLevel())

	// Create a log file for the server logs
	echoLog, err := os.OpenFile(handler.LogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		slog.Error("🚨 🖥️  (cmd/server.go) ❓❓❓❓ 🗒️  Failed to open log file", "error", err)
		return err
	}
	// Write logging output both to Stdout and the log file
	e.Logger.SetOutput(io.MultiWriter(os.Stdout, echoLog))

	// Create an access log
	accessLog, err := os.OpenFile(handler.AccessLogPath(), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		slog.Error("🚨 🖥️  (cmd/server.go) ❓❓❓❓ 🗒️  Failed to open access log file", "error", err)
		return err

	}
	middleware.DefaultLoggerConfig.Output = accessLog

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	slog.Info("✅ 🖥️  (cmd/server.go) 🗒️  Logging configured with", "logLevel", handler.LogLevel(), "logFilePath", handler.LogPath(), "accessLogPath", handler.AccessLogPath())
	return nil
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
