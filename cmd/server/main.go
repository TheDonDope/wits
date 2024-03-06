// Package main is the entry point for the Wits server.
package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/handler"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/joho/godotenv"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	gommonlog "github.com/labstack/gommon/log"
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
	e.File("/favicon.ico", "public/img/favicon.ico")

	configureRoutes(e)

	// Start server
	addr := os.Getenv("HTTP_LISTEN_ADDR")
	slog.Info("🚀 🖥️  (cmd/server.go) 🛜 Wits server is running at", "addr", addr)
	e.Logger.Fatal(e.Start(addr))
}

// configureLogging configures the logging for the server, adding logging and recovery middlewares as well as
// setting the log level from the environment. Finally, it sets the log output to a stdout and file.
func configureLogging(e *echo.Echo) error {
	slog.Info("💬 🖥️  (cmd/server.go) configureLogging()")

	// Set log level from environment variable
	e.Logger.SetLevel(parseLogLevel())

	// Check if log directory exists and create if neccessary
	if _, err := os.Stat(os.Getenv("LOG_DIR")); os.IsNotExist(err) {
		if err := os.Mkdir(os.Getenv("LOG_DIR"), 0755); err != nil {
			slog.Error("🚨 🖥️  (cmd/server.go) ❓❓❓❓ 🗒️  Failed to create log directory", "error", err)
			return err
		}
	}

	// Create a log file for the server logs
	logPath := fmt.Sprintf("%s/%s", os.Getenv("LOG_DIR"), os.Getenv("LOG_FILE"))
	echoLog, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		slog.Error("🚨 🖥️  (cmd/server.go) ❓❓❓❓ 🗒️  Failed to open log file", "error", err)
		return err
	}
	// Write logging output both to Stdout and the log file
	e.Logger.SetOutput(io.MultiWriter(os.Stdout, echoLog))

	// Create an access log
	accessLogPath := fmt.Sprintf("%s/%s", os.Getenv("LOG_DIR"), os.Getenv("ACCESS_LOG_FILE"))
	accessLog, err := os.OpenFile(accessLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0755)
	if err != nil {
		slog.Error("🚨 🖥️  (cmd/server.go) ❓❓❓❓ 🗒️  Failed to open access log file", "error", err)
		return err

	}
	middleware.DefaultLoggerConfig.Output = accessLog

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	slog.Info("✅ 🖥️  (cmd/server.go) configureLogging() -> 🗒️  OK with", "logLevel", os.Getenv("LOG_LEVEL"), "logFilePath", logPath, "accessLogPath", accessLogPath)
	return nil
}

// configureRoutes configures the routes for the server, adding both unprotected and protected routes.
func configureRoutes(e *echo.Echo) {
	// Home Route
	home := handler.HomeHandler{}
	e.GET("/", home.HandleGetHome)

	// Auth routes
	aut := handler.NewAuthHandler()
	e.Use(handler.WithUser())
	e.GET("/login", aut.HandleGetLogin)
	e.GET("/login/provider/google", aut.HandleGetLoginWithGoogle)
	e.POST("/login", aut.HandlePostLogin)
	e.POST("/logout", aut.HandlePostLogout)
	e.GET("/register", aut.HandleGetRegister)
	e.POST("/register", aut.HandlePostRegister)
	e.GET("/auth/callback", aut.HandleGetAuthCallback)

	// Authenticated routes
	indexGroup := e.Group("") // Start with root path
	// Configure middleware with the custom claims type, but only when using local DB
	if os.Getenv("DB_TYPE") == storage.DBTypeLocal {
		indexGroup.Use(echojwt.WithConfig(auth.EchoJWTConfig()))
	}

	indexGroup.Use(handler.WithAuth())

	// Dashboard routes
	dashboard := handler.DashboardHandler{}
	indexGroup.GET("/dashboard", dashboard.HandleGetDashboard)

	// User settings routes
	settings := handler.SettingsHandler{}
	indexGroup.GET("/settings", settings.HandleGetSettings)
}

// initEverything initializes everything needed for the server to run
func initEverything() error {
	if err := godotenv.Load(); err != nil {
		return err
	}

	if err := storage.InitBunWithPostgres(); err != nil {
		return err
	}

	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeRemote {
		return storage.InitSupabaseClient()
	}
	return nil
}

// parseLogLevel returns the log level from the environment, as a log.Lvl
func parseLogLevel() gommonlog.Lvl {
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		return gommonlog.DEBUG
	case "INFO":
		return gommonlog.INFO
	case "WARN":
		return gommonlog.WARN
	case "ERROR":
		return gommonlog.ERROR
	case "OFF":
		return gommonlog.OFF
	default:
		return gommonlog.INFO // Default log level
	}
}
