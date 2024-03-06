package auth

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/sessions"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	// AccessTokenCookieName is the name of the access token cookie.
	AccessTokenCookieName = "wits-access-token"
	// RefreshTokenCookieName is the name of the refresh token cookie.
	RefreshTokenCookieName = "wits-refresh-token"
	// WitsSessionName is the name of the session cookie.
	WitsSessionName = "wits-session"
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
		BeforeFunc:   echoBeforeFunc,
		ErrorHandler: echoJWTErrorHandler,
		SigningKey:   []byte(os.Getenv("JWT_SECRET_KEY")),
		TokenLookupFuncs: []middleware.ValuesExtractor{
			echoContextExtractor,
		},
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(WitsCustomClaims)
		},
	}
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

// echoBeforeFunc sets the access token in the echo.Context.
func echoBeforeFunc(c echo.Context) {
	slog.Info("💬 🏠 (pkg/auth/jwt.go) echoBeforeFunc()")
	store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
	session, _ := store.Get(c.Request(), WitsSessionName)
	accessToken, ok := session.Values[AccessTokenCookieName]
	if !ok {
		slog.Error("🚨 🏠 (pkg/auth/jwt.go) ❓❓❓❓ 🔑 Access token not found in session")
		return
	}
	token := accessToken.(string)
	c.Set(AccessTokenCookieName, token)
	slog.Info("🆗 🏠 (pkg/auth/jwt.go)  🔓 Token found and set with", "token", token[:5]+"...")
	slog.Info("✅ 🏠 (pkg/auth/jwt.go) echoBeforeFunc() -> 📦 Access token has been set in echo.Context")
	return
}

// echoContextExtractor extracts the token from the echo.Context.
func echoContextExtractor(c echo.Context) ([]string, error) {
	slog.Info("💬 🏠 (pkg/auth/jwt.go) echoContextExtractor()")
	result := make([]string, 0)
	if token, ok := c.Get(AccessTokenCookieName).(string); ok {
		result = append(result, token)
	}
	slog.Info("✅ 🏠 (pkg/auth/jwt.go) echoContextExtractor() -> 📦 Token has been extracted from echo.Context with", "token", result[0][:5]+"...")
	return result, nil
}

// echoJWTErrorHandler will be executed when user tries to access a protected path.
func echoJWTErrorHandler(c echo.Context, err error) error {
	slog.Error("🚨 🏠 (pkg/auth/jwt.go) ❓❓❓❓ 🔑 JWT validation failed with", "error", err, "path", c.Request().URL.Path)
	return c.Redirect(http.StatusSeeOther, "/login")
}
