package handler

import (
	"errors"
	"log/slog"
	"os"
	"time"

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
	slog.Info("💬 📖 (pkg/handler/register.go) LocalRegistrator.Register()")
	params := auth.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 📖 (pkg/handler/register.go) ❓❓❓❓ 🔒 Passwords do not match")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}

	// Check if user with email already exists
	existingUser, err := readByEmail(params.Email)
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/register.go) ❓❓❓❓ 🔒 Checking if user exists failed with", "error", err)
	}

	if existingUser != (types.User{}) {
		slog.Error("🚨 📖 (pkg/handler/register.go) ❓❓❓❓ 🔒 User with email already exists")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "User with email already exists",
		}))
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Password), 8)
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/register.go) ❓❓❓❓ 🔒 Hashing password failed with", "error", err)
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
	accessToken, err := signToken(authenticatedUser, []byte(JWTSecret()))
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/register.go) ❓❓❓❓ 🔒 Signing access token failed with", "error", err)
	}
	refreshToken, err := signToken(authenticatedUser, []byte(RefreshJWTSecret()))
	if err != nil {
		slog.Error("🚨 📖 (pkg/handler/register.go) ❓❓❓❓ 🔒 Signing refresh token failed with", "error", err)
	}

	setTokenCookie(AccessTokenCookieName, accessToken, time.Now().Add(1*time.Hour), c)
	setTokenCookie(RefreshTokenCookieName, refreshToken, time.Now().Add(24*time.Hour), c)
	setUserCookie(authenticatedUser, time.Now().Add(1*time.Hour), c)
	slog.Info("✅ 📖 (pkg/handler/register.go) 🔀 User has been registered, redirecting to dashboard")
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
	slog.Info("🆗 🛰️  (pkg/handler/register.go)  🔓 User has been signed up with Supabase with", "email", signUpResp.Email)
	slog.Info("✅ 🛰️  (pkg/handler/register.go) 🔀 User has been registered, redirecting to dashboard")
	return render(c, auth.RegisterSuccess(signUpResp.Email))
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
