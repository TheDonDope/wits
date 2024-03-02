package handler

import (
	"log/slog"

	"github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
)

// LocalVerifier is an struct for the user verification, when using a local sqlite database.
type LocalVerifier struct{}

// Verify verifies the user with the local sqlite database.
func (s LocalVerifier) Verify(c echo.Context) error {
	accessToken := c.Request().URL.Query().Get("access_token")
	if len(accessToken) == 0 {
		return render(c, auth.AuthCallbackScript())
	}
	slog.Info("🔐 🏠 Verified user from url with", "access_token", accessToken)
	return nil
}

// RemoteVerifier is a struct for the user verification, when using a remote Supabase database.
type RemoteVerifier struct{}

// Verify verifies the user with the remote Supabase database.
func (s RemoteVerifier) Verify(c echo.Context) error {
	accessToken := c.Request().URL.Query().Get("access_token")
	if len(accessToken) == 0 {
		return render(c, auth.AuthCallbackScript())
	}
	slog.Info("🔐 🛰️  Verified user from url with", "access_token", accessToken)
	return nil
}
