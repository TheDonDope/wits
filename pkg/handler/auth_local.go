package handler

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	authview "github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

var cookiesToClear []string

func init() {
	cookiesToClear = append(cookiesToClear, types.UserContextKey)
	cookiesToClear = append(cookiesToClear, auth.AccessTokenCookieName)
	cookiesToClear = append(cookiesToClear, auth.RefreshTokenCookieName)
}

// LocalAuthenticator is an interface for the user login, when using a local sqlite database.
type LocalAuthenticator struct{}

// Login logs in the user with the local sqlite database.
func (l LocalAuthenticator) Login(c echo.Context) error {
	slog.Info("💬 🏠 (pkg/handler/auth_local.go) LocalAuthenticator.Login()")
	user, userErr := storage.ReadByEmailAndPassword(c.FormValue("email"), c.FormValue("password"))
	if userErr != nil {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Checking if user exists failed with", "error", userErr)
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	authenticatedUser := types.AuthenticatedUser{
		Email:    user.Email,
		LoggedIn: true,
	}

	// Generate JWT tokens and set cookies 'manually'
	accessToken, err := auth.SignToken(authenticatedUser, []byte(os.Getenv("JWT_SECRET_KEY")))
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Signing access token failed with", "error", err)
	}
	refreshToken, err := auth.SignToken(authenticatedUser, []byte(os.Getenv("JWT_REFRESH_SECRET_KEY")))
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Signing refresh token failed with", "error", err)
	}

	store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	session, _ := store.Get(c.Request(), auth.WitsSessionName)
	session.Values[auth.AccessTokenCookieName] = accessToken
	session.Values[auth.RefreshTokenCookieName] = refreshToken
	session.Values[types.UserContextKey] = authenticatedUser.Email
	cookieErr := session.Save(c.Request(), c.Response())
	if cookieErr != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Saving session failed with", "error", cookieErr)
	}

	slog.Info("🆗 🏠 (pkg/handler/auth_local.go)  🔓 User has been logged in with local Sqlite database")

	slog.Info("✅ 🏠 (pkg/handler/auth_local.go) LocalAuthenticator.Login() -> 🔀 Redirecting to dashboard")
	return hxRedirect(c, "/dashboard")
}

// LocalRegistrator is an interface for the user registration, when using a local sqlite database.
type LocalRegistrator struct{}

// Register logs in the user with the local sqlite database.
func (l LocalRegistrator) Register(c echo.Context) error {
	slog.Info("💬 🏠 (pkg/handler/auth_local.go) LocalRegistrator.Register()")
	params := authview.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Passwords do not match")
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}

	// Check if user with email already exists
	existingUser, err := storage.ReadByEmail(params.Email)
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Checking if user exists failed with", "error", err)
	}

	if existingUser != (types.User{}) {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 User with email already exists")
		return render(c, authview.RegisterForm(params, authview.RegisterErrors{
			InvalidCredentials: "User with email already exists",
		}))
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Password), 8)
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Hashing password failed with", "error", err)
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
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Signing access token failed with", "error", err)
	}
	refreshToken, err := auth.SignToken(authenticatedUser, []byte(os.Getenv("JWT_REFRESH_SECRET_KEY")))
	if err != nil {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Signing refresh token failed with", "error", err)
	}

	store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	session, _ := store.Get(c.Request(), auth.WitsSessionName)
	session.Values[auth.AccessTokenCookieName] = accessToken
	session.Values[auth.RefreshTokenCookieName] = refreshToken
	session.Values[types.UserContextKey] = authenticatedUser.Email
	cookieErr := session.Save(c.Request(), c.Response())
	if cookieErr != nil {
		slog.Error("🚨 🛰️  (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Saving session failed with", "error", cookieErr)
	}

	slog.Info("✅ 🏠 (pkg/handler/auth_local.go) LocalRegistrator.Register() -> 🔀 User has been registered, redirecting to dashboard")
	return hxRedirect(c, "/dashboard")
}

// LocalDeauthenticator is an struct for the user logout, when using a local sqlite database.
type LocalDeauthenticator struct{}

// Logout logs out the user with the local sqlite database.
func (l LocalDeauthenticator) Logout(c echo.Context) error {
	slog.Info("💬 🏠 (pkg/handler/auth_local.go) LocalDeauthenticator.Logout()")

	// Clear cookies from gorilla/sessions store
	store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	session, _ := store.Get(c.Request(), auth.WitsSessionName)
	session.Options.MaxAge = -1
	session.Options.Path = "/"
	session.Values[auth.AccessTokenCookieName] = ""
	session.Values[auth.RefreshTokenCookieName] = ""
	session.Values[types.UserContextKey] = ""
	cookieErr := session.Save(c.Request(), c.Response())
	if cookieErr != nil {
		slog.Error("🚨 🏠 (pkg/handler/auth_local.go) ❓❓❓❓ 🔒 Saving session failed with", "error", cookieErr)
	}

	slog.Info("🆗 🏠 (pkg/handler/auth_local.go)  🎬 User has been logged out")
	slog.Info("✅ 🏠 (pkg/handler/auth_local.go) LocalDeauthenticator.Logout() -> 🔀 Redirecting to login")
	return hxRedirect(c, "/login")
}
