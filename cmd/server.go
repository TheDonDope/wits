// Package main is the entry point for the Wits server.
package main

import (
	"fmt"
	"net/http"

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

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	fmt.Println("Welcome to Wits!")

	db, err := gorm.Open(sqlite.Open("wits.db"), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&types.User{})

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Serve static assets
	e.Static("/assets", "assets")

	// Index Route, redirect to login if necessary
	e.GET("/", handleGetIndex)

	// Login routes
	userStorage := &storage.UserStorage{DB: db}
	userStorage.InsertTestUsers()
	loginHandler := handler.LoginHandler{UserStorage: userStorage}
	e.GET("/login", loginHandler.HandleGetLogin)
	e.POST("/login", loginHandler.HandlePostLogin)

	// Dashboard routes
	r := e.Group("/dashboard")
	// Configure middleware with the custom claims type
	r.Use(echojwt.WithConfig(createEchoJWTConfig()))
	dashboardHandler := handler.DashboardHandler{}
	r.GET("", dashboardHandler.HandleGetDashboard)

	// Start server
	e.Logger.Fatal(e.Start(":3000"))
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
