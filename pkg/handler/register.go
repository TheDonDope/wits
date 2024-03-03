package handler

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
	"github.com/nedpals/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

// LocalRegistrator is an interface for the user registration, when using a local sqlite database.
type LocalRegistrator struct{}

// Register logs in the user with the local sqlite database.
func (s LocalRegistrator) Register(c echo.Context) error {
	slog.Info("💬 🏠 (pkg/handler/register.go) LocalRegistrator.Register()")
	params := auth.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 Passwords do not match")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}

	// Check if user with email already exists
	existingUser, err := readByEmail(params.Email)
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 Checking if user exists failed with", "error", err)
	}

	if existingUser != (types.User{}) {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 User with email already exists")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "User with email already exists",
		}))
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Password), 8)
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 Hashing password failed with", "error", err)
	}

	user := types.User{
		Email:    params.Email,
		Password: string(hashedPassword),
		Name:     params.Username,
	}

	storage.SQLiteDB.Create(&user)

	user.LoggedIn = true

	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔑 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	c.Set(types.UserContextKey, user)
	r := c.Request().WithContext(context.WithValue(c.Request().Context(), types.UserContextKey, user))
	c.SetRequest(r)
	slog.Info("🆗 🏠 (pkg/handler/register.go) 📦 User has been set to context with", "echo.Context.Get(types.UserContextKey)", c.Get(types.UserContextKey), "context.Context.Value(types.UserContextKey)", c.Request().Context().Value(types.UserContextKey))

	slog.Info("✅ 🏠 (pkg/handler/register.go) 🔀 User has been registered, redirecting to dashboard")
	//return render(c, auth.RegisterSuccess(params.Email))
	return hxRedirect(c, "/dashboard")
}

// RemoteRegistrator is an interface for the user registration, when using a remote Supabase database.
type RemoteRegistrator struct{}

// Register logs in the user with the remote Supabase database.
func (s RemoteRegistrator) Register(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/register.go) RemoteRegistrator.Register()")
	params := auth.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🛰️  (pkg/handler/register.go) ❓❓❓❓ 🔒 Passwords do not match")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}
	// Call Supabase to sign up
	signUpResp, err := storage.SupabaseClient.Auth.SignUp(c.Request().Context(), supabase.UserCredentials{Email: params.Email, Password: params.Password})
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/register.go) ❓❓❓❓ 🔒 Signing user up with Supabase failed with", "error", err)
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: err.Error(),
		}))
	}
	slog.Info("🆗 🛰️  (pkg/handler/register.go) 🔓 User has been signed up with Supabase with", "signUpResp", signUpResp)
	slog.Info("✅ 🛰️  (pkg/handler/register.go) 🔀 User has been registered, redirecting to dashboard")
	return render(c, auth.RegisterSuccess(params.Email))
}
