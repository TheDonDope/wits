package handler

import (
	"errors"
	"log/slog"
	"os"
	"time"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	authview "github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
	"github.com/nedpals/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

// LocalRegistrator is an interface for the user registration, when using a local sqlite database.
type LocalRegistrator struct{}

// Register logs in the user with the local sqlite database.
func (s LocalRegistrator) Register(c echo.Context) error {
	slog.Info("💬 🏠 (pkg/handler/register.go) LocalRegistrator.Register()")
	params := authview.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 Passwords do not match")
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}

	// Check if user with email already exists
	existingUser, err := storage.ReadByEmail(params.Email)
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 Checking if user exists failed with", "error", err)
	}

	if existingUser != (types.User{}) {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 User with email already exists")
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
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

	authenticatedUser := types.AuthenticatedUser{
		Email:    user.Email,
		LoggedIn: true}

	// Generate JWT tokens and set cookies 'manually'
	accessToken, err := auth.SignToken(authenticatedUser, []byte(os.Getenv("JWT_SECRET_KEY")))
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 Signing access token failed with", "error", err)
	}
	refreshToken, err := auth.SignToken(authenticatedUser, []byte(os.Getenv("JWT_REFRESH_SECRET_KEY")))
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/register.go) ❓❓❓❓ 🔒 Signing refresh token failed with", "error", err)
	}

	auth.SetTokenCookie(auth.AccessTokenCookieName, accessToken, time.Now().Add(1*time.Hour), c)
	auth.SetTokenCookie(auth.RefreshTokenCookieName, refreshToken, time.Now().Add(24*time.Hour), c)
	auth.SetUserCookie(authenticatedUser, time.Now().Add(1*time.Hour), c)
	slog.Info("✅ 🏠 (pkg/handler/register.go) LocalRegistrator.Register() -> 🔀 User has been registered, redirecting to dashboard")
	return hxRedirect(c, "/dashboard")
}

// RemoteRegistrator is an interface for the user registration, when using a remote Supabase database.
type RemoteRegistrator struct{}

// Register logs in the user with the remote Supabase database.
func (s RemoteRegistrator) Register(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/register.go) RemoteRegistrator.Register()")
	params := authview.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🛰️  (pkg/handler/register.go) ❓❓❓❓ 🔒 Passwords do not match")
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}
	// Call Supabase to sign up
	resp, err := storage.SupabaseClient.Auth.SignUp(c.Request().Context(), supabase.UserCredentials{Email: params.Email, Password: params.Password})
	if err != nil {
		slog.Error("🚨 🛰️  (pkg/handler/register.go) ❓❓❓❓ 🔒 Signing user up with Supabase failed with", "error", err)
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
			InvalidCredentials: err.Error(),
		}))
	}
	slog.Info("🆗 🛰️  (pkg/handler/register.go)  🔓 User has been signed up with Supabase with", "email", resp.Email)
	slog.Info("✅ 🛰️  (pkg/handler/register.go) RemoteRegistrator.Register() -> 🔀 User has been registered, rendering success page")
	return render(c, authview.RegisterSuccess(resp.Email))
}

// NewRegistrator returns a new Registrator based on the DB_TYPE environment variable.
func NewRegistrator() (Registrator, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return &LocalRegistrator{}, nil
	} else if dbType == storage.DBTypeRemote {
		return &RemoteRegistrator{}, nil
	}
	return nil, errors.New("DB_TYPE not set or invalid")
}
