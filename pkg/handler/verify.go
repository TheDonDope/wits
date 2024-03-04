package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
)

// LocalVerifier is an struct for the user verification, when using a local sqlite database.
type LocalVerifier struct{}

// Verify verifies the user with the local sqlite database.
func (s LocalVerifier) Verify(c echo.Context) error {
	slog.Info("💬 📖 (pkg/handler/verify.go) LocalVerifier.Verify()")
	accessToken := c.Request().URL.Query().Get("access_token")
	if len(accessToken) == 0 {
		return render(c, auth.AuthCallbackScript())
	}
	slog.Info("🆗 📖 (pkg/handler/verify.go)  🔑 Parsed URL with access_token")
	SetTokenCookie(AccessTokenCookieName, accessToken, time.Now().Add(1*time.Hour), c)
	return c.Redirect(http.StatusSeeOther, "/")
}

// RemoteVerifier is a struct for the user verification, when using a remote Supabase database.
type RemoteVerifier struct{}

// Verify verifies the user with the remote Supabase database.
func (s RemoteVerifier) Verify(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/verify.go) RemoteVerifier.Verify()")
	accessToken := c.Request().URL.Query().Get("access_token")
	if len(accessToken) == 0 {
		return render(c, auth.AuthCallbackScript())
	}
	slog.Info("🆗 🛰️  (pkg/handler/verify.go)  🔑 Parsed URL with access_token")
	SetTokenCookie(AccessTokenCookieName, accessToken, time.Now().Add(1*time.Hour), c)

	resp, err := storage.SupabaseClient.Auth.User(c.Request().Context(), accessToken)
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/verify.go) ❓❓❓❓ 🔒 Getting user from Supabase failed with", "error", err)
		return nil
	}
	slog.Info("🆗 🛰️  (pkg/handler/verify.go)  🔓 User has been verified with", "email", resp.Email)

	user := types.AuthenticatedUser{
		Email:    resp.Email,
		LoggedIn: true,
	}
	SetUserCookie(user, time.Now().Add(1*time.Hour), c)
	return c.Redirect(http.StatusSeeOther, "/")
}

// NewVerifier returns a new Verifier based on the DB_TYPE environment variable.
func NewVerifier() (Verifier, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return &LocalVerifier{}, nil
	} else if dbType == storage.DBTypeRemote {
		return &RemoteVerifier{}, nil
	}
	return nil, errors.New("DB_TYPE not set or invalid")
}
