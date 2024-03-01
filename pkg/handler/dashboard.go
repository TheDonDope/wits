package handler

import (
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/dashboard"
	"github.com/labstack/echo/v4"
)

// DashboardHandler provides handlers for the dashboard route of the application.
type DashboardHandler struct{}

// HandleGetDashboard responds to GET on the /dashboard route by rendering the Dashboard component.
//
// Parameters:
// - c echo.Context: The echo context.
//
// Returns:
// - error: The error if any.
func (h *DashboardHandler) HandleGetDashboard(c echo.Context) error {
	u, _ := c.Cookie("user")
	slog.Info("🔓 🤝 User cookie found with", "user", u.Value)
	return render(c, dashboard.Dashboard(&types.User{Email: u.Value}))
}
