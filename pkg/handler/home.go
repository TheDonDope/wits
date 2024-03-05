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
	slog.Info("💬 🪴 (pkg/handler/home.go) HandleGetHome()")
	user := getAuthenticatedUser(c)
	if user.LoggedIn {
		slog.Info("🆗 🪴 (pkg/handler/home.go)  💃 User is logged in with", "email", user.Email, "loggedIn", user.LoggedIn)
		slog.Info("✅ 🪴 (pkg/handler/home.go) HandleGetHome() -> 🔀 Redirecting to dashboard")
		return c.Redirect(http.StatusSeeOther, "/dashboard")
	}
	slog.Info("🆗 🪴 (pkg/handler/home.go)  🥷 No User logged in")
	slog.Info("✅ 🪴 (pkg/handler/home.go) HandleGetHome() -> 🔀 Redirecting to login")
	return c.Redirect(http.StatusSeeOther, "/login")
}
