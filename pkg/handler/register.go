package handler

import (
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/labstack/echo/v4"
	"github.com/nedpals/supabase-go"
	"golang.org/x/crypto/bcrypt"
)

// LocalRegisterService is an interface for the user registration, when using a local sqlite database.
type LocalRegisterService struct{}

// Register logs in the user with the local sqlite database.
func (s LocalRegisterService) Register(c echo.Context) error {
	slog.Info("🔐 🏠 Registering user with local sqlite database")
	params := auth.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🤝 Passwords do not match")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}

	// Check if user with email already exists
	existingUser, err := readByEmail(params.Email)
	if err != nil {
		slog.Error("🚨 🤝 Checking if user exists failed with", "error", err)
	}

	if existingUser != nil {
		slog.Error("🚨 🤝 User with email already exists")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "User with email already exists",
		}))
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Password), 8)
	if err != nil {
		slog.Error("🚨 🤝 Hashing password failed with", "error", err)
	}

	user := &types.User{
		Email:    params.Email,
		Password: string(hashedPassword),
		Name:     params.Username,
	}

	storage.SQLiteDB.Create(&user)

	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("✅ 🤝 User has been registered, redirecting to dashboard (reactivate me maybe lol)")
	return render(c, auth.RegisterSuccess(params.Email))
	//return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}

// RemoteRegisterService is an interface for the user registration, when using a remote Supabase database.
type RemoteRegisterService struct{}

// Register logs in the user with the remote Supabase database.
func (s RemoteRegisterService) Register(c echo.Context) error {
	slog.Info("🔐 🛰️  Registering user with remote Supabase database")
	params := auth.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🤝 Passwords do not match")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}
	// Call Supabase to sign up
	signUpResp, err := storage.SupabaseClient.Auth.SignUp(c.Request().Context(), supabase.UserCredentials{Email: params.Email, Password: params.Password})
	if err != nil {
		slog.Error("🚨 🤝 Signing user up with Supabase failed with", "error", err)
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: err.Error(),
		}))
	}
	slog.Info("✅ 🤝 User has been signed up with Supabase with", "signUpResp", signUpResp)

	user := &types.User{
		Email: params.Email,
		Name:  params.Username,
	}

	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("✅ 🤝 User has been registered, redirecting to dashboard (reactivate me maybe lol)")
	return render(c, auth.RegisterSuccess(params.Email))
}
