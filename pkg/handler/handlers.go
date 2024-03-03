package handler

import (
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/a-h/templ"
	"github.com/labstack/echo/v4"
)

// getAuthenticatedUser provides a shorthand function to get the authenticated user from the echo.Context.
func getAuthenticatedUser(c echo.Context) types.User {
	var user types.User
	slog.Info("💬 🤝 (pkg/handler/handlers.go) getAuthenticatedUser")
	u := c.Get(types.UserContextKey)
	if u == nil {
		slog.Error("🚨 🤝 (pkg/handler/handlers.go) ❓❓❓❓ 📦 No User data found in echo.Context, trying with Cookie")
		cookie, err := c.Cookie("user")
		if err != nil {
			slog.Error("🚨 🤝 (pkg/handler/handlers.go) ❓❓❓❓ 🍪 No user cookie found, returning empty user")
			return types.User{}
		}
		slog.Info("✅ 🤝 (pkg/handler/handlers.go) 🍪 User cookie found with", "user", cookie.Value)
		return types.User{
			Email:    cookie.Value,
			LoggedIn: true,
		}
	}
	user = u.(types.User)
	slog.Info("✅ 🤝 (pkg/handler/handlers.go) 📦 User data found in echo.Context with", "email", user.Email, "loggedIn", user.LoggedIn)
	return user
}

// render provides a shorthand function to render the template of a Templ component.
func render(c echo.Context, component templ.Component) error {
	return component.Render(c.Request().Context(), c.Response())
}

// hxRedirect provides a shorthand function to redirect the user with HX-Redirect header.
func hxRedirect(c echo.Context, to string) error {
	slog.Info("💬 🤝 (pkg/handler/handlers.go) 🔄 HTMX-Redirecting", "to", to)
	if len(c.Request().Header.Get("HX-Request")) > 0 {
		c.Response().Header().Set("HX-Redirect", to)
		return nil
	}
	return c.Redirect(http.StatusSeeOther, to)
}
