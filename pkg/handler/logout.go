package handler

import (
	"log/slog"

	"github.com/labstack/echo/v4"
)

// LocalDeauthenticator is an struct for the user logout, when using a local sqlite database.
type LocalDeauthenticator struct{}

// Logout logs out the user with the local sqlite database.
func (s LocalDeauthenticator) Logout(c echo.Context) error {
	slog.Info("🔐 🏠 Logging out user with local sqlite database with", "context", c)
	return nil
}

// RemoteDeauthenticator is a struct for the user logout, when using a remote Supabase database.
type RemoteDeauthenticator struct{}

// Logout logs out the user with the remote Supabase database.
func (s RemoteDeauthenticator) Logout(c echo.Context) error {
	slog.Info("🔐 🛰️  Logging out user with remote Supabase database with", "context", c)
	return nil
}
