package handler

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
	AccessTokenCookieName = "witx-access-token"
	// RefreshTokenCookieName is the name of the refresh token cookie.
	RefreshTokenCookieName = "witx-refresh-token"
)

// WitsCustomClaims are custom claims extending default ones.
// See https://github.com/golang-jwt/jwt for more examples
type WitsCustomClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

// JWTSecret returns the JWT secret key from the environment.
//
// Returns:
// - string: The JWT secret key.
func JWTSecret() string {
	return os.Getenv("JWT_SECRET_KEY")
}

// RefreshJWTSecret returns the refresh JWT secret key from the environment.
//
// Returns:
// - string: The refresh JWT secret key.
func RefreshJWTSecret() string {
	return os.Getenv("JWT_REFRESH_SECRET_KEY")
}

// EchoJWTConfig returns the configuration for the echo-jwt middleware.
//
// Returns:
// - echojwt.Config: The configuration for the echo-jwt middleware.
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
//
// Parameters:
// - err: The error encountered during the HTTP request.
// - c: The echo context.
func HTTPErrorHandler(err error, c echo.Context) {
	slog.Error("🚨 🖥️  HTTP Request failed with", "error", err, "path", c.Request().URL.Path)
}

// JWTErrorHandler will be executed when user tries to access a protected path.
//
// Parameters:
// - c: The echo context.
// - err: The error encountered during JWT validation.
//
// Returns:
// - error: Any error encountered during JWT validation.
func JWTErrorHandler(c echo.Context, err error) error {
	slog.Error("🚨 🏧 JWT validation failed with", "error", err, "path", c.Request().URL.Path)
	return c.Redirect(http.StatusMovedPermanently, "/login")
}

// GenerateTokensAndSetCookies generates a JWT acess and refresh token and set them as cookies for the user,
// as well as the user cookie.
//
// Parameters:
// - user: The user for whom the tokens are being generated.
// - c: The echo context.
//
// Returns:
// - error: Any error encountered during token generation.
func GenerateTokensAndSetCookies(user *types.User, c echo.Context) error {
	accessToken, exp, err := generateAccessToken(user)
	if err != nil {
		slog.Error("🚨 🏧 Generating access token failed with", "error", err, "path", c.Request().URL.Path)
		return err
	}

	setTokenCookie(AccessTokenCookieName, accessToken, exp, c)
	setUserCookie(user, exp, c)
	refreshToken, exp, err := generateRefreshToken(user)
	if err != nil {
		slog.Error("🚨 🏧 Generating refresh token failed with", "error", err, "path", c.Request().URL.Path)
		return err
	}
	setTokenCookie(RefreshTokenCookieName, refreshToken, exp, c)
	slog.Info("🔑 🏧 Tokens have been generated and set", "path", c.Request().URL.Path)
	return nil
}

// generateToken generates a JWT token for the given user with the specified expiration time.
// It signs the token using the provided secret and returns the token string, expiration time, and any error encountered.
//
// Parameters:
// - user: The user for whom the token is being generated.
// - expirationTime: The expiration time for the token.
// - secret: The secret key used for signing the token.
//
// Returns:
// - string: The generated JWT token string.
// - time.Time: The expiration time of the token.
// - error: Any error encountered during token generation.
func generateToken(user *types.User, expirationTime time.Time, secret []byte) (string, time.Time, error) {
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
		slog.Error("🚨 🏧 Signing token failed with", "error", err)
		return "", time.Now(), err
	}

	return tokenString, expirationTime, nil
}

// generateAccessToken generates an access token for the user.
//
// Parameters:
// - user: The user for whom the token is being generated.
//
// Returns:
// - string: The generated JWT token string.
// - time.Time: The expiration time of the token.
// - error: Any error encountered during token generation.
func generateAccessToken(user *types.User) (string, time.Time, error) {
	// Declare the expiration time of the token
	expirationTime := time.Now().Add(1 * time.Hour)

	return generateToken(user, expirationTime, []byte(JWTSecret()))
}

// generateRefreshToken generates a refresh token for the user.
//
// Parameters:
// - user: The user for whom the token is being generated.
//
// Returns:
// - string: The generated JWT token string.
// - time.Time: The expiration time of the token.
// - error: Any error encountered during token generation.
func generateRefreshToken(user *types.User) (string, time.Time, error) {
	// Declare the expiration time of the token
	expirationTime := time.Now().Add(24 * time.Hour)

	return generateToken(user, expirationTime, []byte(RefreshJWTSecret()))
}

// setTokenCookie sets a cookie with the given name, token, expiration time, and echo.Context.
// The cookie is set with the specified name, value, expiration time, and path ("/").
// It is also set to be accessible only through HTTP (HttpOnly).
//
// Parameters:
// - name: The name of the cookie.
// - token: The value of the cookie.
// - expiration: The expiration time for the cookie.
// - c: The echo context.
func setTokenCookie(name, token string, expiration time.Time, c echo.Context) {
	cookie := new(http.Cookie)
	cookie.Name = name
	cookie.Value = token
	cookie.Expires = expiration
	cookie.Path = "/"
	cookie.HttpOnly = true
	c.SetCookie(cookie)
	slog.Info("🍪 🏧 Cookie has been set with", "name", name, "value", token)
}

// setUserCookie sets a cookie with the user's email as the value.
// It also logs the cookie information using slog.Info.
//
// Parameters:
// - user: The user for whom the cookie is being generated.
// - expiration: The expiration time for the cookie.
// - c: The echo context.
func setUserCookie(user *types.User, expiration time.Time, c echo.Context) {
	cookie := new(http.Cookie)
	cookie.Name = "user"
	cookie.Value = user.Email
	cookie.Expires = expiration
	cookie.Path = "/"
	c.SetCookie(cookie)
	slog.Info("🍪 🏧 Cookie has been set with", "name", "user", "value", user.Email)
}
