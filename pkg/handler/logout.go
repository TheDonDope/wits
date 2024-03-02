package handler

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// LocalDeauthenticator is an struct for the user logout, when using a local sqlite database.
type LocalDeauthenticator struct{}

// Logout logs out the user with the local sqlite database.
func (s LocalDeauthenticator) Logout(c echo.Context) error {
	slog.Info("🔐 🏠 Logging out user with local sqlite database with", "context", c)
	userCookie := &http.Cookie{
		Name:   AccessTokenCookieName,
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	}
	c.SetCookie(userCookie)
	slog.Info("🔀 🤝 Redirecting to login")
	return c.Redirect(http.StatusSeeOther, "/login")
}

// RemoteDeauthenticator is a struct for the user logout, when using a remote Supabase database.
type RemoteDeauthenticator struct{}

// Logout logs out the user with the remote Supabase database.
func (s RemoteDeauthenticator) Logout(c echo.Context) error {
	slog.Info("🔐 🛰️  Logging out user with remote Supabase database with", "context", c)
	userCookie := &http.Cookie{
		Name:   AccessTokenCookieName,
		Value:  "",
		MaxAge: -1,
		Path:   "/",
	}
	c.SetCookie(userCookie)
	slog.Info("🔀 🤝 Redirecting to login")
	return c.Redirect(http.StatusSeeOther, "/login")
}
