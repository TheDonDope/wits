package handler

import (
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/dashboard"
	"github.com/labstack/echo/v4"
)

// DashboardHandler handles the dashboard page.
type DashboardHandler struct{}

// HandleGetDashboard responds to GET on the /dashboard route by rendering the Dashboard component.
func (h *DashboardHandler) HandleGetDashboard(c echo.Context) error {
	userCookie, _ := c.Cookie("user")
	slog.Info("🔓 User cookie found", "user", userCookie.Value)
	return render(c, dashboard.Dashboard(&types.User{Email: userCookie.Value}))
}
