package handler

import (
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/view/dashboard"
	"github.com/labstack/echo/v4"
)

// DashboardHandler provides handlers for the dashboard route of the application.
type DashboardHandler struct{}

// HandleGetDashboard responds to GET on the /dashboard route by rendering the Dashboard component.
func (h *DashboardHandler) HandleGetDashboard(c echo.Context) error {
	slog.Info("💬 🤝 (pkg/handler/dashboard.go) HandleGetDashboard()")
	user := getAuthenticatedUser(c)
	if user.LoggedIn {
		slog.Info("🆗 🤝 (pkg/handler/dashboard.go) 📦 User is logged in with", "email", user.Email, "loggedIn", user.LoggedIn)
		slog.Info("✅ 🤝 (pkg/handler/dashboard.go) 🔀 Redirecting to dashboard")
		return render(c, dashboard.Dashboard(user))
	}
	slog.Info("🆗 🤝 (pkg/handler/dashboard.go) 📦 No User logged")
	slog.Info("✅ 🤝 (pkg/handler/dashboard.go) 🔀 Redirecting to login")
	return c.Redirect(http.StatusSeeOther, "/login")
}
