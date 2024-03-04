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
	"github.com/nedpals/supabase-go"
)

// LocalAuthenticator is an interface for the user login, when using a local sqlite database.
type LocalAuthenticator struct{}

// Login logs in the user with the local sqlite database.
func (s LocalAuthenticator) Login(c echo.Context) error {
	slog.Info("💬 📖 (pkg/handler/login.go) LocalAuthenticator.Login()")
	user, userErr := readByEmailAndPassword(c.FormValue("email"), c.FormValue("password"))
	if userErr != nil {
		slog.Error("🚨 📖 (pkg/handler/login.go) ❓❓❓❓ 🔒 Checking if user exists failed with", "error", userErr)
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	authenticatedUser := types.AuthenticatedUser{
		Email:    user.Email,
		LoggedIn: true,
	}

	// Generate JWT tokens and set cookies 'manually'
	accessToken, err := signToken(authenticatedUser, []byte(JWTSecret()))
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/login.go) ❓❓❓❓ 🔒 Signing access token failed with", "error", err)
	}
	refreshToken, err := signToken(authenticatedUser, []byte(RefreshJWTSecret()))
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/login.go) ❓❓❓❓ 🔒 Signing refresh token failed with", "error", err)
	}

	setTokenCookie(AccessTokenCookieName, accessToken, time.Now().Add(1*time.Hour), c)
	setTokenCookie(RefreshTokenCookieName, refreshToken, time.Now().Add(24*time.Hour), c)
	setUserCookie(authenticatedUser, time.Now().Add(1*time.Hour), c)

	slog.Info("🆗 📖 (pkg/handler/login.go)  🔓 User has been logged in with local Sqlite database")

	slog.Info("✅ 📖 (pkg/handler/login.go) 🔀 Redirecting to dashboard")
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
	signInResp, err := storage.SupabaseClient.Auth.SignIn(c.Request().Context(), credentials)
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/login.go) ❓❓❓❓ 🔒 Signing user in with Supabase failed with", "error", err)
		return render(c, auth.LoginForm(credentials, auth.LoginErrors{
			InvalidCredentials: "The credentials you have entered are invalid",
		}))
	}
	slog.Info("🆗 🛰️  (pkg/handler/login.go)  🔓 User has been logged in with", "signInResp", signInResp)

	authenticatedUser := types.AuthenticatedUser{
		Email:    signInResp.User.Email,
		LoggedIn: true,
	}

	setTokenCookie(AccessTokenCookieName, signInResp.AccessToken, time.Now().Add(1*time.Hour), c)
	setTokenCookie(RefreshTokenCookieName, signInResp.RefreshToken, time.Now().Add(24*time.Hour), c)
	setUserCookie(authenticatedUser, time.Now().Add(1*time.Hour), c)

	slog.Info("✅ 🛰️  (pkg/handler/login.go) 🔀 Redirecting to dashboard")
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
	slog.Info("💬 🛰️  (pkg/handler/login.go) GoogleAuthenticator.Login()", "path", c.Request().URL.Path)
	resp, err := storage.SupabaseClient.Auth.SignInWithProvider(supabase.ProviderSignInOptions{
		Provider:   "google",
		RedirectTo: AuthCallbackURL(),
	})
	if err != nil {
		return err
	}
	slog.Info("🆗 🛰️  (pkg/handler/login.go)  🔓 User has been logged in with Google", "resp", resp)
	slog.Info("✅ 🛰️  (pkg/handler/login.go) 🔀 Redirecting to", "url", resp.URL)
	return c.Redirect(http.StatusSeeOther, resp.URL)
}

// NewGoogleAuthenticator returns a new GoogleAuthenticator.
func NewGoogleAuthenticator() (GoogleAuthenticator, error) {
	return GoogleAuthenticator{}, nil
}
