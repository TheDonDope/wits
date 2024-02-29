package handler

import (
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
func JWTSecret() string {
	return os.Getenv("JWT_SECRET_KEY")
}

// RefreshJWTSecret returns the refresh JWT secret key from the environment.
func RefreshJWTSecret() string {
	return os.Getenv("JWT_REFRESH_SECRET_KEY")
}

// EchoJWTConfig returns the configuration for the echo-jwt middleware.
func EchoJWTConfig() echojwt.Config {
	return echojwt.Config{
		NewClaimsFunc: func(c echo.Context) jwt.Claims {
			return new(WitsCustomClaims)
		},
		SigningKey:   []byte(JWTSecret()),
		TokenLookup:  "cookie:witx-access-token",
		ErrorHandler: JWTErrorChecker,
	}
}

// JWTErrorChecker will be executed when user try to access a protected path.
func JWTErrorChecker(c echo.Context, err error) error {
	slog.Error("🚨 🏧 JWT Error", "error", err, "path", c.Request().URL.Path)
	return c.Redirect(http.StatusMovedPermanently, "/login")
}

// GenerateTokensAndSetCookies generates tokens and sets cookies for the user.
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

// generateToken generates a JWT token for the user.
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
func generateAccessToken(user *types.User) (string, time.Time, error) {
	// Declare the expiration time of the token
	expirationTime := time.Now().Add(1 * time.Hour)

	return generateToken(user, expirationTime, []byte(JWTSecret()))
}

// generateRefreshToken generates a refresh token for the user.
func generateRefreshToken(user *types.User) (string, time.Time, error) {
	// Declare the expiration time of the token
	expirationTime := time.Now().Add(24 * time.Hour)

	return generateToken(user, expirationTime, []byte(RefreshJWTSecret()))
}

// setTokenCookie sets a token cookie.
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

// setUserCookie sets a user cookie.
func setUserCookie(user *types.User, expiration time.Time, c echo.Context) {
	cookie := new(http.Cookie)
	cookie.Name = "user"
	cookie.Value = user.Email
	cookie.Expires = expiration
	cookie.Path = "/"
	c.SetCookie(cookie)
	slog.Info("🍪 🏧 Cookie has been set with", "name", "user", "value", user.Email)
}
