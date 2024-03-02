package handler

import (
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
	"github.com/nedpals/supabase-go"
)

// LocalLoginService is an interface for the user login, when using a local sqlite database.
type LocalLoginService struct{}

// Login logs in the user with the local sqlite database.
func (s LocalLoginService) Login(c echo.Context) error {
	slog.Info("🔐 🏠 Logging in user with local sqlite database")
	user, userErr := readByEmailAndPassword(c.FormValue("email"), c.FormValue("password"))
	if userErr != nil {
		slog.Error("🚨 🤝 Checking if user exists failed with", "error", userErr)
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	// Generate JWT tokens and set cookies 'manually'
	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("🔀 🤝 Redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}

// RemoteLoginService is an interface for the user login, when using a remote Supabase database.
type RemoteLoginService struct{}

// Login logs in the user with the remote Supabase database.
func (s RemoteLoginService) Login(c echo.Context) error {
	slog.Info("🔐 🛰️  Logging in user with remote Supabase database")
	credentials := supabase.UserCredentials{
		Email:    c.FormValue("email"),
		Password: c.FormValue("password"),
	}

	// Call Supabase to sign in
	signInResp, err := storage.SupabaseClient.Auth.SignIn(c.Request().Context(), credentials)
	if err != nil {
		slog.Error("🚨 🤝 Signing user in with Supabase failed with", "error", err)
		return render(c, auth.LoginForm(credentials, auth.LoginErrors{
			InvalidCredentials: "The credentials you have entered are invalid",
		}))
	}
	slog.Info("✅ 🤝 User has been logged in with", "signInResp", signInResp)

	// Checkme:
	c.SetCookie(&http.Cookie{
		Value:    signInResp.AccessToken,
		Name:     "at",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	})

	user := &types.User{
		Email: signInResp.User.Email,
		Name:  signInResp.User.ID,
	}

	// Generate JWT tokens and set cookies 'manually'
	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("🔀 🤝 Redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}
