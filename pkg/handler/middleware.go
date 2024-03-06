package handler

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
)

// HTTPErrorHandler will be executed when an HTTP request fails.
func HTTPErrorHandler(err error, c echo.Context) {
	slog.Error("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🛜 HTTP Request failed with", "error", err, "path", c.Request().URL.Path)
}

// WithUser is a middleware that sets the user in the request context.
func WithUser() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if strings.Contains(c.Request().URL.Path, "/public") || strings.Contains(c.Request().URL.Path, "/favicon.ico") {
				return next(c)
			}
			slog.Info("💬 🏧 (pkg/handler/middleware.go) WithUser() -> next()", "path", c.Request().URL.Path)

			// Get the authenticatedUser from the request context
			var authenticatedUser types.AuthenticatedUser

			store := sessions.NewCookieStore([]byte(os.Getenv("SESSION_SECRET")))
			session, _ := store.Get(c.Request(), WitsSessionName)
			if session.Values[types.UserContextKey] != nil {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go)  🍪 User found in session with", "name", types.UserContextKey, "value", session.Values[types.UserContextKey])
				authenticatedUser = types.AuthenticatedUser{
					Email:    session.Values[types.UserContextKey].(string),
					LoggedIn: true,
				}
			} else {
				// Kept for backwards compatibility with local cookies
				userContext := c.Get(types.UserContextKey)
				if userContext == nil {
					slog.Info("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 📦 No user data found in echo.Context, trying with Cookie. Looked for", "contextKey", types.UserContextKey)
					userCookie, err := c.Cookie(types.UserContextKey)
					if err != nil {
						slog.Info("🚨 🏧 (pkg/handler/middleware.go) ❓❓❓❓ 🍪 No user cookie found, returning empty user. Looked for", "cookieName", types.UserContextKey)
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
			if strings.Contains(c.Request().URL.Path, "/public") || strings.Contains(c.Request().URL.Path, "/favicon.ico") {
				return next(c)
			}
			slog.Info("💬 🏧 (pkg/handler/middleware.go) WitAuth() -> next()", "path", c.Request().URL.Path)
			user := getAuthenticatedUser(c)
			if !user.LoggedIn {
				slog.Info("🆗 🏧 (pkg/handler/middleware.go)  🥷 No authorized user found")
				slog.Info("✅ 🏧 (pkg/handler/middleware.go) WitAuth() -> next() -> 🔀 Redirecting to login")
				return c.Redirect(http.StatusSeeOther, "/login?to="+c.Request().URL.Path)
			}
			slog.Info("🆗 🏧 (pkg/handler/middleware.go)  💃 Authorized user found with", "email", user.Email)
			slog.Info("✅ 🏧 (pkg/handler/middleware.go) WitAuth() -> next() -> 💫 Continuing navigation", "to", c.Request().URL.Path)
			return next(c)
		}
	}
}
