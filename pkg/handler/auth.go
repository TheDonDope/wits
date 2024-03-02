package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/labstack/echo/v4"
)

// Authenticator is an interface for the authentication service.
type Authenticator interface {
	// Login logs in the user
	Login(c echo.Context) error
}

// Deauthenticator is an interface for the deauthentication service.
type Deauthenticator interface {
	// Logout logs out the user
	Logout(c echo.Context) error
}

// Registrator is an interface for the registration service.
type Registrator interface {
	// Register registers the user
	Register(c echo.Context) error
}

// AuthHandler provides handlers for the authentication routes of the application.
// It is responsible for handling user login, registration, and logout.
type AuthHandler struct {
	a Authenticator
	d Deauthenticator
	r Registrator
}

// NewAuthHandler creates a new AuthHandler with the given LoginService and RegisterService, depending on the database type.
func NewAuthHandler() *AuthHandler {
	dbType := os.Getenv("DB_TYPE")
	var a Authenticator
	var d Deauthenticator
	var r Registrator
	if dbType == storage.DBTypeLocal {
		a = LocalAuthenticator{}
		d = LocalDeauthenticator{}
		r = LocalRegistrator{}
	} else if dbType == storage.DBTypeRemote {
		a = RemoteAuthenticator{}
		d = RemoteDeauthenticator{}
		r = RemoteRegistrator{}
	}
	return &AuthHandler{a: a, d: d, r: r}
}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
func (h AuthHandler) HandleGetLogin(c echo.Context) error {
	return render(c, auth.Login())
}

// HandlePostLogin responds to POST on the /login route by trying to log in the user.
// If the user exists and the password is correct, the JWT tokens are generated and set as cookies.
// Finally, the user is redirected to the dashboard.
func (h AuthHandler) HandlePostLogin(c echo.Context) error {
	slog.Info("🔐 🤝 Logging in user")
	return h.a.Login(c)
}

// HandlePostLogout responds to POST on the /logout route by logging out the user.
func (h AuthHandler) HandlePostLogout(c echo.Context) error {
	slog.Info("🔐 🤝 Logging out user")
	return h.d.Logout(c)
}

// HandleGetRegister responds to GET on the /register route by rendering the Register component.
func (h AuthHandler) HandleGetRegister(c echo.Context) error {
	return render(c, auth.Register())
}

// HandlePostRegister responds to POST on the /register route by trying to register the user.
// If the user does not exist, the password is hashed and the user is created in the database.
// Afterwards, the JWT tokens are generated and set as cookies. Finally, the user is redirected to the dashboard.
func (h AuthHandler) HandlePostRegister(c echo.Context) error {
	slog.Info("🔐 🤝 Registering user")
	return h.r.Register(c)
}

// readByEmailAndPassword returns a user with the given email and password.
func readByEmailAndPassword(email string, password string) (*types.User, error) {
	user, err := readByEmail(email)
	if err != nil {
		slog.Error("🚨 📁 Finding user by email failed with", "error", err)
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		slog.Error("🚨 📁 Password is incorrect")
		return nil, fmt.Errorf("Password is incorrect")
	}

	return user, nil
}

// readByEmail returns a user with the given email.
func readByEmail(email string) (*types.User, error) {
	var user types.User
	err := storage.SQLiteDB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("🚨 📁 Finding user failed with", "error", err)
		return nil, err
	}

	return &user, nil
}
