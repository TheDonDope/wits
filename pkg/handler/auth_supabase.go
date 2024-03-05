package handler

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	authview "github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
	"github.com/nedpals/supabase-go"
)

// SupabaseAuthenticator is an interface for the user login, when using a remote Supabase database.
type SupabaseAuthenticator struct{}

// Login logs in the user with the remote Supabase database.
func (s SupabaseAuthenticator) Login(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/auth_supabase.go) SupabaseAuthenticator.Login()")
	credentials := supabase.UserCredentials{
		Email:    c.FormValue("email"),
		Password: c.FormValue("password"),
	}

	// Call Supabase to sign in
	resp, err := storage.SupabaseClient.Auth.SignIn(c.Request().Context(), credentials)
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Signing user in with Supabase failed with", "error", err)
		return render(c, authview.LoginForm(credentials, authview.LoginErrors{
			InvalidCredentials: "The credentials you have entered are invalid",
		}))
	}
	slog.Info("🆗 🛰️  (pkg/handler/auth_supabase.go)  🔓 User has been logged in with", "resp", resp)

	authenticatedUser := types.AuthenticatedUser{
		Email:    resp.User.Email,
		LoggedIn: true,
	}

	auth.SetTokenCookie(auth.AccessTokenCookieName, resp.AccessToken, time.Now().Add(1*time.Hour), c)
	auth.SetTokenCookie(auth.RefreshTokenCookieName, resp.RefreshToken, time.Now().Add(24*time.Hour), c)
	auth.SetUserCookie(authenticatedUser, time.Now().Add(1*time.Hour), c)

	slog.Info("✅ 🛰️  (pkg/handler/auth_supabase.go) SupabaseAuthenticator.Login() -> 🔀 Redirecting to dashboard")
	return hxRedirect(c, "/dashboard")
}

// SupabaseRegistrator is an interface for the user registration, when using a remote Supabase database.
type SupabaseRegistrator struct{}

// Register logs in the user with the remote Supabase database.
func (s SupabaseRegistrator) Register(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/auth_supabase.go) SupabaseRegistrator.Register()")
	params := authview.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Passwords do not match")
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}
	// Call Supabase to sign up
	resp, err := storage.SupabaseClient.Auth.SignUp(c.Request().Context(), supabase.UserCredentials{Email: params.Email, Password: params.Password})
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Signing user up with Supabase failed with", "error", err)
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
			InvalidCredentials: err.Error(),
		}))
	}
	slog.Info("🆗 🛰️  (pkg/handler/auth_supabase.go)  🔓 User has been signed up with Supabase with", "email", resp.Email)
	slog.Info("✅ 🛰️  (pkg/handler/auth_supabase.go) SupabaseRegistrator.Register() -> 🔀 User has been registered, rendering success page")
	return render(c, authview.RegisterSuccess(resp.Email))
}

// SupabaseVerifier is a struct for the user verification, when using a remote Supabase database.
type SupabaseVerifier struct{}

// Verify verifies the user with the remote Supabase database.
func (s SupabaseVerifier) Verify(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/auth_supabase.go) SupabaseVerifier.Verify()")
	accessToken := c.Request().URL.Query().Get("access_token")
	if len(accessToken) == 0 {
		return render(c, authview.AuthCallbackScript())
	}
	slog.Info("🆗 🛰️  (pkg/handler/auth_supabase.go)  🔑 Parsed URL with access_token")
	auth.SetTokenCookie(auth.AccessTokenCookieName, accessToken, time.Now().Add(1*time.Hour), c)

	resp, err := storage.SupabaseClient.Auth.User(c.Request().Context(), accessToken)
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Getting user from Supabase failed with", "error", err)
		return nil
	}
	slog.Info("🆗 🛰️  (pkg/handler/auth_supabase.go)  🔓 User has been verified with", "email", resp.Email)

	user := types.AuthenticatedUser{
		Email:    resp.Email,
		LoggedIn: true,
	}
	auth.SetUserCookie(user, time.Now().Add(1*time.Hour), c)
	slog.Info("✅ 🏠 (pkg/handler/auth_supabase.go) SupabaseVerifier.Verify() -> 🔀 Redirecting to index")
	return c.Redirect(http.StatusSeeOther, "/")
}
