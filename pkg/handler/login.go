package handler

import (
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/view/login"
	"github.com/labstack/echo/v4"
)

// LoginHandler ...
type LoginHandler struct {
	Users *storage.UserStorage
}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
func (h LoginHandler) HandleGetLogin(c echo.Context) error {
	return render(c, login.Login())
}

// HandlePostLogin responds to POST on the /login route by ...
func (h LoginHandler) HandlePostLogin(c echo.Context) error {
	slog.Info("🔐 🤝 Logging in user")
	email := c.FormValue("email")
	password := c.FormValue("password")

	user, userErr := h.Users.GetUserByEmailAndPassword(email, password)

	if userErr != nil {
		slog.Error("🚨 🤝 Checking if user exists failed with", "error", userErr)
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	tokenErr := auth.GenerateTokensAndSetCookies(user, c)

	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("✅ 🤝 User has been logged in, redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")

}
