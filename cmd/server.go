// Package main is the entry point for the Wits server.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/handler"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/joho/godotenv"
)

func main() {
	fmt.Println("Welcome to Wits!")

	if err := initEverything(); err != nil {
		log.Fatal(err)
	}

	db, err := gorm.Open(sqlite.Open(os.Getenv("DATA_SOURCE_NAME")), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&types.User{})

	userStorage := &storage.UserStorage{DB: db}

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Serve static assets
	e.Static("/assets", "assets")

	// Index Route, redirect to login if necessary
	e.GET("/", handleGetIndex)

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
	r.Use(echojwt.WithConfig(createEchoJWTConfig()))
	dashboardHandler := handler.DashboardHandler{}
	r.GET("", dashboardHandler.HandleGetDashboard)

	// Start server
	e.Logger.Fatal(e.Start(os.Getenv("HTTP_LISTEN_ADDR")))
}

func initEverything() error {
	// if err := godotenv.Load(); err != nil {
	// 	return err
	// }
	return godotenv.Load()
}

func createEchoJWTConfig() echojwt.Config {
	return echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(auth.WitsCustomClaims)
		},
		SigningKey:   []byte(auth.GetJWTSecret()),
		TokenLookup:  "cookie:witx-access-token",
		ErrorHandler: auth.JWTErrorChecker,
	}
}

func handleGetIndex(c echo.Context) error {
	_, err := c.Cookie("user")
	if err != nil {
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}
