package handler

import (
	"log/slog"

	"github.com/TheDonDope/wits/pkg/view/settings" // Import the missing package
	"github.com/labstack/echo/v4"
)

// SettingsHandler provides handlers for the settings route of the application.
type SettingsHandler struct{}

// HandleGetSettings responds to GET on the /settings route by rendering the settings page.
func (h SettingsHandler) HandleGetSettings(c echo.Context) error {
	slog.Info("💬 🛠️  (pkg/handler/settings.go) HandleGetSettings()")
	user := getAuthenticatedUser(c)
	return render(c, settings.Index(user))
}
