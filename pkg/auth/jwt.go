package auth

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
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

// JWTErrorHandler will be executed when user tries to access a protected path.
func JWTErrorHandler(c echo.Context, err error) error {
	slog.Error("🚨 🏠 (pkg/auth/jwt.go) ❓❓❓❓ 🔑 JWT validation failed with", "error", err, "path", c.Request().URL.Path)
	return c.Redirect(http.StatusSeeOther, "/login")
}

// SignToken signs a JWT token for the given user with the specified secret.
func SignToken(user types.AuthenticatedUser, secret []byte) (string, error) {
	slog.Info("💬 🏠 (pkg/auth/jwt.go) SignToken()")
	claims := &WitsCustomClaims{
		Email: user.Email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(1 * time.Hour)),
		},
	}
	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Return the signed JWT string
	slog.Info("✅ 🏠 (pkg/auth/jwt.go) SignToken() -> 🔑 Token has been signed for", "email", user.Email)
	return token.SignedString(secret)
}

// SetTokenCookie sets a cookie with the given name, token, expiration time, and echo.Context.
// The cookie is set with the specified name, value, expiration time, and path ("/").
// It is also set to be accessible only through HTTP (HttpOnly).
func SetTokenCookie(name, token string, expiration time.Time, c echo.Context) {
	slog.Info("💬 🏠 (pkg/auth/jwt.go) SetTokenCookie()")
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = "/"
	cookie.HttpOnly = true
	c.SetCookie(cookie)
	slog.Info("✅ 🏠 (pkg/auth/jwt.go) SetTokenCookie() -> 🍪 Cookie has been set with", "name", name, "value", token[:5]+"...")
}

// SetUserCookie sets a cookie with the user's email as the value.
func SetUserCookie(user types.AuthenticatedUser, expiration time.Time, c echo.Context) {
	slog.Info("💬 🏠 (pkg/auth/jwt.go) SetUserCookie()")
	cookie := new(http.Cookie)
	cookie.Name = types.UserContextKey
	cookie.Value = user.Email
	cookie.Expires = expiration
	cookie.Path = "/"
	c.SetCookie(cookie)
	slog.Info("✅ 🏠 (pkg/auth/jwt.go) SetUserCookie() -> 🍪 Cookie has been set with", "name", types.UserContextKey, "value", user.Email)
}
