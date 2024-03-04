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

// Authenticator is the interface that wraps the basic Login method.
type Authenticator interface {
	// Login signs in the user with the application
	Login(c echo.Context) error
}

// Deauthenticator is the interface that wraps the basic Logout method.
type Deauthenticator interface {
	// Logout logs out the user
	Logout(c echo.Context) error
}

// Registrator is the interface that wraps the basic Register method.
type Registrator interface {
	// Register registers the user
	Register(c echo.Context) error
}

// Verifier is the interface that wraps the basic Verify method.
type Verifier interface {
	// Verify verifies the user
	Verify(c echo.Context) error
}

// AuthHandler provides handlers for the authentication routes of the application.
// It is responsible for handling user login, registration, and logout.
type AuthHandler struct {
	a Authenticator
	g Authenticator
	d Deauthenticator
	r Registrator
	v Verifier
}

// NewAuthHandler creates a new AuthHandler with the given LoginService and RegisterService, depending on the database type.
func NewAuthHandler() *AuthHandler {
	dbType := os.Getenv("DB_TYPE")
	var a Authenticator
	var g Authenticator
	var d Deauthenticator
	var r Registrator
	var v Verifier
	if dbType == storage.DBTypeLocal {
		a = LocalAuthenticator{}
		d = LocalDeauthenticator{}
		r = LocalRegistrator{}
		v = LocalVerifier{}
	} else if dbType == storage.DBTypeRemote {
		a = RemoteAuthenticator{}
		d = RemoteDeauthenticator{}
		r = RemoteRegistrator{}
		v = RemoteVerifier{}
	}
	g = GoogleAuthenticator{}
	return &AuthHandler{a: a, g: g, d: d, r: r, v: v}
}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
func (h AuthHandler) HandleGetLogin(c echo.Context) error {
	slog.Info("✅ 🔒 (pkg/handler/auth.go) HandleGetLogin()")
	return render(c, auth.Login())
}

// HandlePostLogin responds to POST on the /login route by trying to log in the user.
// If the user exists and the password is correct, the JWT tokens are generated and set as cookies.
// Finally, the user is redirected to the dashboard.
func (h AuthHandler) HandlePostLogin(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandlePostLogin()")
	return h.a.Login(c)
}

// HandleGetLoginWithGoogle responds to GET on the /login/provider/google route by logging in the user with Google.
func (h AuthHandler) HandleGetLoginWithGoogle(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandleGetLoginWithGoogle()")
	return h.g.Login(c)
}

// HandlePostLogout responds to POST on the /logout route by logging out the user.
func (h AuthHandler) HandlePostLogout(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandlePostLogout()")
	return h.d.Logout(c)
}

// HandleGetRegister responds to GET on the /register route by rendering the Register component.
func (h AuthHandler) HandleGetRegister(c echo.Context) error {
	slog.Info("✅ 🔒 (pkg/handler/auth.go) HandleGetRegister()")
	return render(c, auth.Register())
}

// HandlePostRegister responds to POST on the /register route by trying to register the user.
// If the user does not exist, the password is hashed and the user is created in the database.
// Afterwards, the JWT tokens are generated and set as cookies. Finally, the user is redirected to the dashboard.
func (h AuthHandler) HandlePostRegister(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandlePostRegister()")
	return h.r.Register(c)
}

// HandleGetAuthCallback responds to GET on the /auth/callback route by verifying the user.
func (h AuthHandler) HandleGetAuthCallback(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandleGetAuthCallback()")
	return h.v.Verify(c)
}

// readByEmailAndPassword returns a user with the given email and password.
func readByEmailAndPassword(email string, password string) (types.User, error) {
	slog.Info("💬 🔒 (pkg/handler/auth.go) readByEmailAndPassword()")
	user, err := readByEmail(email)
	if err != nil {
		slog.Error("🚨 🔒 (pkg/handler/auth.go) ❓❓❓❓ 📖 Finding user by email failed with", "error", err)
		return types.User{}, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		slog.Error("🚨 🔒 (pkg/handler/auth.go) ❓❓❓❓ 📖 Password is incorrect")
		return types.User{}, fmt.Errorf("(pkg/handler/auth.go) Password is incorrect")
	}

	return user, nil
}

// readByEmail returns a user with the given email.
func readByEmail(email string) (types.User, error) {
	slog.Info("💬 🔒 (pkg/handler/auth.go) readByEmail()")
	var user types.User
	err := storage.SQLiteDB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("🚨 🔒 (pkg/handler/auth.go) ❓❓❓❓ 📖 Finding user failed with", "error", err)
		return types.User{}, err
	}

	return user, nil
}
