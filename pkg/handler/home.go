package handler

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// HomeHandler is a handler for the root route
type HomeHandler struct{}

// HandleHomeIndex responds to GET on the / route by redirecting to the dashboard if the user is logged in,
// otherwise to the login page.
func (h *HomeHandler) HandleHomeIndex(c echo.Context) error {
	_, err := c.Cookie("user")
	if err != nil {
		slog.Info("🔐 🤝 No user cookie found, redirecting to login")
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	slog.Info("🔓 🤝 User cookie found, redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}
