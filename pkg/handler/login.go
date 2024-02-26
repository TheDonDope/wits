package handler

import (
	"github.com/TheDonDope/wits/pkg/view/login"
	"github.com/labstack/echo/v4"
)

// LoginHandler ...
type LoginHandler struct{}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
func (h LoginHandler) HandleGetLogin(c echo.Context) error {
	return render(c, login.Login())
}
