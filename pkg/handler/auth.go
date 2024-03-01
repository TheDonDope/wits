package handler

import (
	"log/slog"
	"net/http"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

// AuthHandler provides handlers for the authentication routes of the application.
// It is responsible for handling user login, registration, and logout.
type AuthHandler struct {
	Users *storage.UserStorage
}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
//
// Parameters:
// - c echo.Context: The echo context.
//
// Returns:
// - error: The error if any.
func (h AuthHandler) HandleGetLogin(c echo.Context) error {
	return render(c, auth.Login())
}

// HandlePostLogin responds to POST on the /login route by trying to log in the user.
// If the user exists and the password is correct, the JWT tokens are generated and set as cookies.
// Finally, the user is redirected to the dashboard.
//
// Parameters:
// - c echo.Context: The echo context.
//
// Returns:
// - error: The error if any.
func (h AuthHandler) HandlePostLogin(c echo.Context) error {
	slog.Info("🔐 🤝 Logging in user")
	email := c.FormValue("email")
	password := c.FormValue("password")

	user, userErr := h.Users.GetUserByEmailAndPassword(email, password)

	if userErr != nil {
		slog.Error("🚨 🤝 Checking if user exists failed with", "error", userErr)
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	tokenErr := GenerateTokensAndSetCookies(user, c)

	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("✅ 🤝 User has been logged in, redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}

// HandleGetRegister responds to GET on the /register route by rendering the Register component.
//
// Parameters:
// - c echo.Context: The echo context.
//
// Returns:
// - error: The error if any.
func (h AuthHandler) HandleGetRegister(c echo.Context) error {
	return render(c, auth.Register())
}

// HandlePostRegister responds to POST on the /register route by trying to register the user.
// If the user does not exist, the password is hashed and the user is created in the database.
// Afterwards, the JWT tokens are generated and set as cookies. Finally, the user is redirected to the dashboard.
//
// Parameters:
// - c echo.Context: The echo context.
//
// Returns:
// - error: The error if any.
func (h AuthHandler) HandlePostRegister(c echo.Context) error {
	slog.Info("🔐 🤝 Registering user")
	username := c.FormValue("username")
	email := c.FormValue("email")
	password := c.FormValue("password")
	passwordConfirm := c.FormValue("password-confirmation")

	if password != passwordConfirm {
		slog.Error("🚨 🤝 Passwords do not match")
		return echo.NewHTTPError(http.StatusBadRequest, "Passwords do not match")
	}

	// Check if user with email already exists
	existingUser, err := h.Users.GetUserByEmail(email)
	if err != nil {
		slog.Error("🚨 🤝 Checking if user exists failed with", "error", err)
	}

	if existingUser != nil {
		slog.Error("🚨 🤝 User with email already exists")
		return echo.NewHTTPError(http.StatusBadRequest, "User with email already exists")
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 8)
	if err != nil {
		slog.Error("🚨 🤝 Hashing password failed with", "error", err)
	}

	user := &types.User{
		Email:    email,
		Password: string(hashedPassword),
		Name:     username,
	}

	h.Users.DB.Create(&user)

	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("✅ 🤝 User has been registered, redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}
