package handler

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/golang-jwt/jwt/v5"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
)

const (
	// AccessTokenCookieName is the name of the access token cookie.
	AccessTokenCookieName = "wits-access-token"
	// RefreshTokenCookieName is the name of the refresh token cookie.
	RefreshTokenCookieName = "wits-refresh-token"
)

// WitsCustomClaims are custom claims extending default ones.
// See https://github.com/golang-jwt/jwt for more examples
type WitsCustomClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// JWTSecret returns the JWT secret key from the environment.
func JWTSecret() string {
	return os.Getenv("JWT_SECRET_KEY")
}

// RefreshJWTSecret returns the refresh JWT secret key from the environment.
func RefreshJWTSecret() string {
	return os.Getenv("JWT_REFRESH_SECRET_KEY")
}

// AuthCallbackURL returns the authentication callback URL from the environment.
func AuthCallbackURL() string {
	return os.Getenv("AUTH_CALLBACK_URL")
}

// EchoJWTConfig returns the configuration for the echo-jwt middleware.
func EchoJWTConfig() echojwt.Config {
	return echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(WitsCustomClaims)
		},
		SigningKey:   []byte(JWTSecret()),
		TokenLookup:  fmt.Sprintf("cookie:%s", AccessTokenCookieName),
		ErrorHandler: JWTErrorHandler,
	}
}

// HTTPErrorHandler will be executed when an HTTP request fails.
func HTTPErrorHandler(err error, c echo.Context) {
	slog.Error("🚨 🖥️  (pkg/handler/middleware.go) ❓❓❓❓ 🛜 HTTP Request failed with", "error", err, "path", c.Request().URL.Path)
}

// JWTErrorHandler will be executed when user tries to access a protected path.
func JWTErrorHandler(c echo.Context, err error) error {
	slog.Error("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🔑 JWT validation failed with", "error", err, "path", c.Request().URL.Path)
	return c.Redirect(http.StatusSeeOther, "/login")
}

// WithUser is a middleware that sets the user in the request context.
func WithUser() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.Contains(c.Request().URL.Path, "/public") {
				return next(c)
			}
			slog.Info("💬 🏧 (pkg/handler/middleware.go) WithUser() -> next()", "path", c.Request().URL.Path)

			// Get the authenticatedUser from the request context
			var authenticatedUser types.AuthenticatedUser
			userContext := c.Get(types.UserContextKey)
			if userContext == nil {
				slog.Debug("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 📦 No user data found in echo.Context, trying with Cookie. Looked for", "contextKey", types.UserContextKey)
				userCookie, err := c.Cookie(types.UserContextKey)
				if err != nil {
					slog.Debug("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🍪 No user cookie found, returning empty user. Looked for", "cookieName", types.UserContextKey)
					authenticatedUser = types.AuthenticatedUser{}
				} else {
					slog.Info("🆗 🏧 (pkg/handler/middleware.go) 🍪 User cookie found with", "name", types.UserContextKey, "value", userCookie.Value)
					authenticatedUser = types.AuthenticatedUser{
						Email:    userCookie.Value,
						LoggedIn: true,
					}
				}
			} else {
				user := userContext.(types.AuthenticatedUser)
				authenticatedUser = types.AuthenticatedUser{
					Email:    user.Email,
					LoggedIn: true,
				}
			}
			if len(authenticatedUser.Email) == 0 && !authenticatedUser.LoggedIn {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go) 🥷 Empty, unauthorized user data found in echo.Context with", "contextKey", types.UserContextKey, "email", authenticatedUser.Email)
			} else {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go) 💃 User data found in echo.Context with", "contextKey", types.UserContextKey, "email", authenticatedUser.Email)
			}
			// Set the user in the echo.Context
			c.Set(types.UserContextKey, authenticatedUser)
			// Set the user in the context.Context
			r := c.Request().WithContext(context.WithValue(c.Request().Context(), types.UserContextKey, authenticatedUser))
			c.SetRequest(r)
			if len(authenticatedUser.Email) == 0 && !authenticatedUser.LoggedIn {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go) 🥷 Empty, unauthorized has been set to context with", "echo.Context.Get(types.UserContextKey)", c.Get(types.UserContextKey), "context.Context.Value(types.UserContextKey)", c.Request().Context().Value(types.UserContextKey))
				slog.Info("✅ 🏧 (pkg/handler/middleware.go) WithUser() -> next() -> 🥷 Empty, unauthorized found in echo.Context with", "path", c.Request().URL.Path, "contextKey", types.UserContextKey, "email", authenticatedUser.Email)
			} else {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go) 💃 User has been set to context with", "echo.Context.Get(types.UserContextKey)", c.Get(types.UserContextKey), "context.Context.Value(types.UserContextKey)", c.Request().Context().Value(types.UserContextKey))
				slog.Info("✅ 🏧 (pkg/handler/middleware.go) WithUser() -> next() -> 💃 User found in echo.Context with", "path", c.Request().URL.Path, "contextKey", types.UserContextKey, "email", authenticatedUser.Email)
			}

			return next(c)
		}
	}
}

// GenerateTokensAndSetCookies generates a JWT acess and refresh token and set them as cookies for the user,
// as well as the user cookie.
func GenerateTokensAndSetCookies(user types.AuthenticatedUser, c echo.Context) error {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) GenerateTokensAndSetCookies()")
	accessToken, exp, err := generateAccessToken(user)
	if err != nil {
		slog.Error("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🔑 Generating access token failed with", "error", err, "path", c.Request().URL.Path)
		return err
	}

	SetTokenCookie(AccessTokenCookieName, accessToken, exp, c)
	SetUserCookie(user, exp, c)
	refreshToken, exp, err := generateRefreshToken(user)
	if err != nil {
		slog.Error("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🔑 Generating refresh token failed with", "error", err, "path", c.Request().URL.Path)
		return err
	}
	SetTokenCookie(RefreshTokenCookieName, refreshToken, exp, c)
	slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🔑 Tokens have been generated and set", "path", c.Request().URL.Path)
	return nil
}

// SetTokenCookie sets a cookie with the given name, token, expiration time, and echo.Context.
// The cookie is set with the specified name, value, expiration time, and path ("/").
// It is also set to be accessible only through HTTP (HttpOnly).
func SetTokenCookie(name, token string, expiration time.Time, c echo.Context) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) SetTokenCookie()")
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = "/"
	cookie.HttpOnly = true
	c.SetCookie(cookie)
	slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🍪 Cookie has been set with", "name", name, "value", token)
}

// SetUserCookie sets a cookie with the user's email as the value.
func SetUserCookie(user types.AuthenticatedUser, expiration time.Time, c echo.Context) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) SetUserCookie()")
	cookie := new(http.Cookie)
	cookie.Name = types.UserContextKey
	cookie.Value = user.Email
	cookie.Expires = expiration
	cookie.Path = "/"
	c.SetCookie(cookie)
	slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🍪 Cookie has been set with", "name", types.UserContextKey, "value", user.Email)
}

// generateToken generates a JWT token for the given user with the specified expiration time.
// It signs the token using the provided secret and returns the token string, expiration time, and any error encountered.
func generateToken(user types.AuthenticatedUser, expirationTime time.Time, secret []byte) (string, time.Time, error) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) generateToken()")
	// Create the JWT claims, which includes the username and expiry time
	claims := &WitsCustomClaims{
		user.Email,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
		},
	}

	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Create the JWT string
	tokenString, err := token.SignedString(secret)
	if err != nil {
		slog.Error("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🔑 Signing token failed with", "error", err)
		return "", time.Now(), err
	}

	slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🔑 Token has been signed for", "email", user.Email)
	return tokenString, expirationTime, nil
}

// generateAccessToken generates an access token for the user.
func generateAccessToken(user types.AuthenticatedUser) (string, time.Time, error) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) generateAccessToken()")
	// Declare the expiration time of the token
	expirationTime := time.Now().Add(1 * time.Hour)

	return generateToken(user, expirationTime, []byte(JWTSecret()))
}

// generateRefreshToken generates a refresh token for the user.
func generateRefreshToken(user types.AuthenticatedUser) (string, time.Time, error) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) generateRefreshToken()")
	// Declare the expiration time of the token
	expirationTime := time.Now().Add(24 * time.Hour)

	return generateToken(user, expirationTime, []byte(RefreshJWTSecret()))
}
