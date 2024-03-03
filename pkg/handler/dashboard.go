package handler

import (
	"log/slog"

	"github.com/TheDonDope/wits/pkg/view/dashboard"
	"github.com/labstack/echo/v4"
)

// DashboardHandler provides handlers for the dashboard route of the application.
type DashboardHandler struct{}

// HandleGetDashboard responds to GET on the /dashboard route by rendering the Dashboard component.
func (h *DashboardHandler) HandleGetDashboard(c echo.Context) error {
	slog.Info("💬 🤝 (pkg/handler/dashboard.go) HandleGetDashboard")
	user := getAuthenticatedUser(c)
	u, _ := c.Cookie("user")
	slog.Info("✅ 🤝 (pkg/handler/dashboard) 🍪 User cookie found with", "user", u.Value)
	return render(c, dashboard.Dashboard(user))
}
