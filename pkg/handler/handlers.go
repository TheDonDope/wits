package handler

import (
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// getAuthenticatedUser provides a shorthand function to get the authenticated user from the echo.Context.
func getAuthenticatedUser(c echo.Context) types.AuthenticatedUser {
	var user types.AuthenticatedUser
	slog.Info("💬 🤝 (pkg/handler/handlers.go) getAuthenticatedUser()", "path", c.Request().URL.Path)
	u := c.Get(types.UserContextKey)
	if u == nil {
		slog.Debug("🚨 🤝 (pkg/handler/handlers.go) ❓❓❓❓ 📦 No user data found in echo.Context, trying with Cookie. Looked for", "contextKey", types.UserContextKey)
		cookie, err := c.Cookie(types.UserContextKey)
		if err != nil {
			slog.Info("✅ 🤝 (pkg/handler/handlers.go) ❓❓❓❓ 🍪 No user cookie found, returning empty user. Looked for", "cookieName", types.UserContextKey)
			return types.AuthenticatedUser{}
		}
		slog.Info("✅ 🤝 (pkg/handler/handlers.go) 🍪 User cookie found with", "name", types.UserContextKey, "value", cookie.Value)
		return types.AuthenticatedUser{
			Email:    cookie.Value,
			LoggedIn: true,
		}
	}
	user = u.(types.AuthenticatedUser)
	slog.Info("✅ 🤝 (pkg/handler/handlers.go) 📦 User data found in echo.Context with", "contextKey", types.UserContextKey, "email", user.Email, "loggedIn", user.LoggedIn)
	return user
}

// render provides a shorthand function to render the template of a Templ component.
func render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response())
}

// hxRedirect provides a shorthand function to redirect the user with HX-Redirect header.
func hxRedirect(c echo.Context, to string) error {
	slog.Info("💬 🤝 (pkg/handler/handlers.go) 🔄 hxRedirect()", "to", to)
	if len(c.Request().Header.Get("HX-Request")) > 0 {
		c.Response().Header().Set("HX-Redirect", to)
		return nil
	}
	return c.Redirect(http.StatusSeeOther, to)
}
