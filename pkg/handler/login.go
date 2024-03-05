package handler

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	authview "github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
	"github.com/nedpals/supabase-go"
)

// LocalAuthenticator is an interface for the user login, when using a local sqlite database.
type LocalAuthenticator struct{}

// Login logs in the user with the local sqlite database.
func (s LocalAuthenticator) Login(c echo.Context) error {
	slog.Info("💬 📖 (pkg/handler/login.go) LocalAuthenticator.Login()")
	user, userErr := storage.ReadByEmailAndPassword(c.FormValue("email"), c.FormValue("password"))
	if userErr != nil {
		slog.Error("🚨 📖 (pkg/handler/login.go) ❓❓❓❓ 🔒 Checking if user exists failed with", "error", userErr)
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	authenticatedUser := types.AuthenticatedUser{
		Email:    user.Email,
		LoggedIn: true,
	}

	// Generate JWT tokens and set cookies 'manually'
	accessToken, err := auth.SignToken(authenticatedUser, []byte(os.Getenv("JWT_SECRET_KEY")))
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/login.go) ❓❓❓❓ 🔒 Signing access token failed with", "error", err)
	}
	refreshToken, err := auth.SignToken(authenticatedUser, []byte(os.Getenv("JWT_REFRESH_SECRET_KEY")))
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/login.go) ❓❓❓❓ 🔒 Signing refresh token failed with", "error", err)
	}

	auth.SetTokenCookie(auth.AccessTokenCookieName, accessToken, time.Now().Add(1*time.Hour), c)
	auth.SetTokenCookie(auth.RefreshTokenCookieName, refreshToken, time.Now().Add(24*time.Hour), c)
	auth.SetUserCookie(authenticatedUser, time.Now().Add(1*time.Hour), c)

	slog.Info("🆗 📖 (pkg/handler/login.go)  🔓 User has been logged in with local Sqlite database")

	slog.Info("✅ 📖 (pkg/handler/login.go) LocalAuthenticator.Login() -> 🔀 Redirecting to dashboard")
	return hxRedirect(c, "/dashboard")
}

// RemoteAuthenticator is an interface for the user login, when using a remote Supabase database.
type RemoteAuthenticator struct{}

// Login logs in the user with the remote Supabase database.
func (s RemoteAuthenticator) Login(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/login.go) RemoteAuthenticator.Login()")
	credentials := supabase.UserCredentials{
		Email:    c.FormValue("email"),
		Password: c.FormValue("password"),
	}

	// Call Supabase to sign in
	resp, err := storage.SupabaseClient.Auth.SignIn(c.Request().Context(), credentials)
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/login.go) ❓❓❓❓ 🔒 Signing user in with Supabase failed with", "error", err)
		return render(c, authview.LoginForm(credentials, authview.LoginErrors{
			InvalidCredentials: "The credentials you have entered are invalid",
		}))
	}
	slog.Info("🆗 🛰️  (pkg/handler/login.go)  🔓 User has been logged in with", "resp", resp)

	authenticatedUser := types.AuthenticatedUser{
		Email:    resp.User.Email,
		LoggedIn: true,
	}

	auth.SetTokenCookie(auth.AccessTokenCookieName, resp.AccessToken, time.Now().Add(1*time.Hour), c)
	auth.SetTokenCookie(auth.RefreshTokenCookieName, resp.RefreshToken, time.Now().Add(24*time.Hour), c)
	auth.SetUserCookie(authenticatedUser, time.Now().Add(1*time.Hour), c)

	slog.Info("✅ 🛰️  (pkg/handler/login.go) RemoteAuthenticator.Login() -> 🔀 Redirecting to dashboard")
	return hxRedirect(c, "/dashboard")
}

// NewAuthenticator returns the correct Authenticator based on the DB_TYPE environment variable.
func NewAuthenticator() (Authenticator, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return &LocalAuthenticator{}, nil
	} else if dbType == storage.DBTypeRemote {
		return &RemoteAuthenticator{}, nil
	}
	return nil, errors.New("DB_TYPE not set or invalid")
}

// GoogleAuthenticator is an interface for the user login, when using Google.
type GoogleAuthenticator struct{}

// Login logs in the user with their Google Credentials
func (s GoogleAuthenticator) Login(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/login.go) GoogleAuthenticator.Login()")
	resp, err := storage.SupabaseClient.Auth.SignInWithProvider(supabase.ProviderSignInOptions{
		Provider:   "google",
		RedirectTo: os.Getenv("AUTH_CALLBACK_URL"),
	})
	if err != nil {
		return err
	}
	slog.Info("🆗 🛰️  (pkg/handler/login.go)  🔓 User has been logged in with Google", "resp", resp)
	slog.Info("✅ 🛰️  (pkg/handler/login.go) RemoteAuthenticator.Login() -> 🔀 Redirecting to", "url", resp.URL)
	return c.Redirect(http.StatusSeeOther, resp.URL)
}

// NewGoogleAuthenticator returns a new GoogleAuthenticator.
func NewGoogleAuthenticator() (GoogleAuthenticator, error) {
	return GoogleAuthenticator{}, nil
}
