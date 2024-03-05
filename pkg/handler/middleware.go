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

// EchoJWTConfig returns the configuration for the echo-jwt middleware.
func EchoJWTConfig() echojwt.Config {
	return echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(WitsCustomClaims)
		},
		SigningKey:   []byte(os.Getenv("JWT_SECRET_KEY")),
		TokenLookup:  fmt.Sprintf("cookie:%s", AccessTokenCookieName),
		ErrorHandler: JWTErrorHandler,
	}
}

// HTTPErrorHandler will be executed when an HTTP request fails.
func HTTPErrorHandler(err error, c echo.Context) {
	slog.Error("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🛜 HTTP Request failed with", "error", err, "path", c.Request().URL.Path)
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
					slog.Info("🆗 🏧 (pkg/handler/middleware.go)  🍪 User cookie found with", "name", types.UserContextKey, "value", userCookie.Value)
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
			// Set the user in the echo.Context
			c.Set(types.UserContextKey, authenticatedUser)
			// Set the user in the context.Context
			r := c.Request().WithContext(context.WithValue(c.Request().Context(), types.UserContextKey, authenticatedUser))
			c.SetRequest(r)

			if len(authenticatedUser.Email) == 0 && !authenticatedUser.LoggedIn {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go)  🥷 Empty, unauthorized user has been set to echo.Context and echo.Context.Request().Context()")
				slog.Info("✅ 🏧 (pkg/handler/middleware.go) WithUser() -> next() -> 🥷 Empty, unauthorized user found in echo.Context with", "path", c.Request().URL.Path)
			} else {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go)  💃 User has been set to to echo.Context and echo.Context.Request().Context()")
				slog.Info("✅ 🏧 (pkg/handler/middleware.go) WithUser() -> next() -> 💃 User found in echo.Context with", "path", c.Request().URL.Path)
			}

			return next(c)
		}
	}
}

// WithAuth is a middleware that checks if the user is authenticated.
func WithAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.Contains(c.Request().URL.Path, "/public") {
				return next(c)
			}
			slog.Info("💬 🏧 (pkg/handler/middleware.go) WitAuth() -> next()", "path", c.Request().URL.Path)
			user := getAuthenticatedUser(c)
			if !user.LoggedIn {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go)  🥷 No authorized user found")
				slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🔀 Redirecting to login")
				return c.Redirect(http.StatusSeeOther, "/login?to="+c.Request().URL.Path)
			}
			slog.Info("🆗 🏧 (pkg/handler/middleware.go)  💃 Authorized user found with", "email", user.Email)
			slog.Info("✅ 🏧 (pkg/handler/middleware.go) WitAuth() -> next() -> 💫 Continuing navigation", "to", c.Request().URL.Path)
			return next(c)
		}
	}
}

// signToken signs a JWT token for the given user with the specified secret.
func signToken(user types.AuthenticatedUser, secret []byte) (string, error) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) SignToken()")
	claims := &WitsCustomClaims{
		user.Email,
		jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Return the signed JWT string
	slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🔑 Token has been signed for", "email", user.Email)
	return token.SignedString(secret)
}

// setTokenCookie sets a cookie with the given name, token, expiration time, and echo.Context.
// The cookie is set with the specified name, value, expiration time, and path ("/").
// It is also set to be accessible only through HTTP (HttpOnly).
func setTokenCookie(name, token string, expiration time.Time, c echo.Context) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) SetTokenCookie()")
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = "/"
	cookie.HttpOnly = true
	c.SetCookie(cookie)
	slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🍪 Cookie has been set with", "name", name, "value", token[:5]+"...")
}

// setUserCookie sets a cookie with the user's email as the value.
func setUserCookie(user types.AuthenticatedUser, expiration time.Time, c echo.Context) {
	slog.Info("💬 🏧 (pkg/handler/middleware.go) SetUserCookie()")
	cookie := new(http.Cookie)
	cookie.Name = types.UserContextKey
	cookie.Value = user.Email
	cookie.Expires = expiration
	cookie.Path = "/"
	c.SetCookie(cookie)
	slog.Info("✅ 🏧 (pkg/handler/middleware.go) 🍪 Cookie has been set with", "name", types.UserContextKey, "value", user.Email)
}
