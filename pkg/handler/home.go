package handler

import (
	"log/slog"
	"net/http"

	"github.com/labstack/echo/v4"
)

// HomeHandler provides handlers for the home route of the application.
type HomeHandler struct{}

// HandleGetHome responds to GET on the / route by redirecting to the dashboard if the user is logged in,
// otherwise to the login page.
func (h *HomeHandler) HandleGetHome(c echo.Context) error {
	slog.Info("💬 🤝 (pkg/handler/home.go) HandleGetHome()")
	_, err := c.Cookie("user")
	if err != nil {
		slog.Error("🚨 🤝 (pkg/handler/home.go) ❓❓❓❓ 🍪 No user cookie found, redirecting to login")
		return c.Redirect(http.StatusSeeOther, "/login")
	}
	slog.Info("✅ 🤝 (pkg/handler/home.go) 🍪 User cookie found, redirecting to dashboard")
	return c.Redirect(http.StatusSeeOther, "/dashboard")
}
