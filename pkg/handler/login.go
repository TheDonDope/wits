package handler

import (
	"net/http"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/login"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// LoginHandler ...
type LoginHandler struct{}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
func (h LoginHandler) HandleGetLogin(c echo.Context) error {
	return render(c, login.Login())
}

// HandlePostLogin responds to POST on the /login route by ...
func (h LoginHandler) HandlePostLogin(c echo.Context) error {
	email := c.FormValue("email")
	passwd := c.FormValue("password")

	user := &types.User{
		ID:       uuid.NewString(),
		Email:    email,
		Password: passwd,
	}

	// Throws unauthorized error
	if user.Email != "foo@bar.org" || user.Password != "known" {
		return echo.ErrUnauthorized
	}

	err := auth.GenerateTokensAndSetCookies(user, c)

	if err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	return c.Redirect(http.StatusMovedPermanently, "/dashboard")

}
