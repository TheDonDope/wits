package main

import (
	"fmt"
	"net/http"

	"github.com/TheDonDope/wits/pkg/handler"
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
	e.GET("/", func(c echo.Context) error {
		return c.Redirect(http.StatusSeeOther, "/login")
	})

	// Login routes
	loginHandler := handler.LoginHandler{}
	e.GET("/login", loginHandler.HandleGetLogin)

	// Start server
	e.Logger.Fatal(e.Start(":3000"))
}
