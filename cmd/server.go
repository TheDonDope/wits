// Package main is the entry point for the Wits server.
package main

import (
	"fmt"
	"net/http"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/handler"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	fmt.Println("Welcome to Wits!")

	e := echo.New()

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Serve static assets
	e.Static("/assets", "assets")

	// Index Route, redirect to login if necessary
	e.GET("/", handleGetIndex)

	// Login routes
	loginHandler := handler.LoginHandler{}
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
