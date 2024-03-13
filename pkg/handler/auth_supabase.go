package handler

import (
	"encoding/gob"
	"log/slog"
	"net/http"
	"os"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	authview "github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/google/uuid"
	"github.com/gorilla/sessions"
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
	resp, sessionErr := storage.SupabaseClient.Auth.SignIn(c.Request().Context(), credentials)
	if sessionErr != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Signing user in with Supabase failed with", "error", sessionErr)
		return render(c, authview.LoginForm(credentials.Email, credentials.Password, authview.LoginErrors{
			InvalidCredentials: "The credentials you have entered are invalid",
		}))
	}
	slog.Info("🆗 🛰️  (pkg/handler/auth_supabase.go)  🔓 User has been logged in with", "email", resp.User.Email)

	authenticatedUser := types.AuthenticatedUser{
		ID:       uuid.MustParse(resp.User.ID),
		Email:    resp.User.Email,
		LoggedIn: true,
	}

	// Register uuid.UUID with gob
	gob.Register(uuid.UUID{})

	store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	session, _ := store.Get(c.Request(), auth.WitsSessionName)
	session.Values[auth.AccessTokenCookieName] = resp.AccessToken
	session.Values[auth.RefreshTokenCookieName] = resp.RefreshToken
	session.Values[types.UserContextKey] = authenticatedUser.Email
	session.Values[types.UserIdKey] = authenticatedUser.ID
	cookieErr := session.Save(c.Request(), c.Response())
	if cookieErr != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Saving session failed with", "error", cookieErr)
	}

	slog.Info("✅ 🛰️  (pkg/handler/auth_supabase.go) SupabaseAuthenticator.Login() -> 🔀 Redirecting to dashboard")
	return hxRedirect(c, "/dashboard")
}

// SupabaseRegistrator is an interface for the user registration, when using a remote Supabase database.
type SupabaseRegistrator struct{}

// Register logs in the user with the remote Supabase database.
func (s SupabaseRegistrator) Register(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/auth_supabase.go) SupabaseRegistrator.Register()")
	params := authview.RegisterParams{
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

	resp, err := storage.SupabaseClient.Auth.User(c.Request().Context(), accessToken)
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Getting user from Supabase failed with", "error", err)
		return nil
	}
	slog.Info("🆗 🛰️  (pkg/handler/auth_supabase.go)  🔓 User has been verified with", "email", resp.Email)

	// Register uuid.UUID with gob
	gob.Register(uuid.UUID{})

	store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	session, _ := store.Get(c.Request(), auth.WitsSessionName)
	session.Values[auth.AccessTokenCookieName] = accessToken
	session.Values[types.UserContextKey] = resp.Email
	session.Values[types.UserIdKey] = resp.ID
	cookieErr := session.Save(c.Request(), c.Response())
	if cookieErr != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_supabase.go) ❓❓❓❓ 🔒 Saving session failed with", "error", cookieErr)
	}

	slog.Info("✅ 🛰️ (pkg/handler/auth_supabase.go) SupabaseVerifier.Verify() -> 🔀 Redirecting to index")
	return c.Redirect(http.StatusSeeOther, "/")
}
