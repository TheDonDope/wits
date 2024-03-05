package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/labstack/echo/v4"
)

var cookiesToClear []string

// LocalDeauthenticator is an struct for the user logout, when using a local sqlite database.
type LocalDeauthenticator struct{}

func init() {
	cookiesToClear = append(cookiesToClear, types.UserContextKey)
	cookiesToClear = append(cookiesToClear, auth.AccessTokenCookieName)
	cookiesToClear = append(cookiesToClear, auth.RefreshTokenCookieName)
}

// Logout logs out the user with the local sqlite database.
func (s LocalDeauthenticator) Logout(c echo.Context) error {
	slog.Info("💬 📖 (pkg/handler/logout.go) LocalDeauthenticator.Logout()")

	// Clear all cookies
	for _, cookieName := range cookiesToClear {
		cookie := &http.Cookie{
			Name:   cookieName,
			Value:  "",
			MaxAge: -1,
			Path:   "/",
		}
		c.SetCookie(cookie)
		slog.Info("🆗 📖 (pkg/handler/logout.go)  🗑️  Cookie cleared with", "cookie", cookie)
	}
	slog.Info("🆗 📖 (pkg/handler/logout.go)  🎬 User has been logged out")
	slog.Info("✅ 📖 (pkg/handler/logout.go) 🔀 Redirecting to login")
	return hxRedirect(c, "/login")
}

// RemoteDeauthenticator is a struct for the user logout, when using a remote Supabase database.
type RemoteDeauthenticator struct{}

// Logout logs out the user with the remote Supabase database.
func (s RemoteDeauthenticator) Logout(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/logout.go) RemoteDeauthenticator.Logout()")
	// Clear all cookies
	for _, cookieName := range cookiesToClear {
		cookie := &http.Cookie{
			Name:   cookieName,
			Value:  "",
			MaxAge: -1,
			Path:   "/",
		}
		c.SetCookie(cookie)
		slog.Info("🆗 🛰️  (pkg/handler/logout.go)  🗑️  Cookie cleared with", "cookie", cookie)
	}
	slog.Info("🆗 🛰️  (pkg/handler/logout.go)  🎬 User has been logged out")
	slog.Info("✅ 🛰️  (pkg/handler/logout.go) 🔀 Redirecting to login")
	return hxRedirect(c, "/login")
}

// NewDeauthenticator returns a new Deauthenticator based on the DB_TYPE environment variable.
func NewDeauthenticator() (Deauthenticator, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return &LocalDeauthenticator{}, nil
	} else if dbType == storage.DBTypeRemote {
		return &RemoteDeauthenticator{}, nil
	}
	return nil, errors.New("DB_TYPE not set or invalid")
}
