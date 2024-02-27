package handler

import (
	"fmt"
	"net/http"

	"github.com/TheDonDope/wits/pkg/auth"
	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/register"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

// RegisterHandler ...
type RegisterHandler struct {
	UserStorage *storage.UserStorage
}

// HandleGetRegister responds to GET on the /register route by rendering the Register component.
func (h RegisterHandler) HandleGetRegister(c echo.Context) error {
	return render(c, register.Register())
}

// HandlePostRegister responds to POST on the /register route by ...
func (h RegisterHandler) HandlePostRegister(c echo.Context) error {
	username := c.FormValue("username")
	email := c.FormValue("email")
	password := c.FormValue("password")
	passwordConfirm := c.FormValue("password-confirmation")

	if password != passwordConfirm {
		return echo.NewHTTPError(http.StatusBadRequest, "Passwords do not match")
	}

	// Check if user with email already exists
	existingUser, err := h.UserStorage.GetUserByEmail(email)
	if err != nil {
		fmt.Println("Error checking if user exists")
	}

	if existingUser != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "User with email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 8)
	if err != nil {
		fmt.Println("Error hashing password")
	}

	user := &types.User{
		Email:    email,
		Password: string(hashedPassword),
		Name:     username,
	}

	h.UserStorage.DB.Create(&user)

	tokenErr := auth.GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}
