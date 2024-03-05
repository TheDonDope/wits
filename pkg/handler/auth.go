package handler

import (
	"errors"
	"log/slog"
	"os"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"
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

// NewAuthenticator returns the correct Authenticator based on the DB_TYPE environment variable.
func NewAuthenticator() (Authenticator, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return &LocalAuthenticator{}, nil
	} else if dbType == storage.DBTypeRemote {
		return &SupabaseAuthenticator{}, nil
	}
	return nil, errors.New("DB_TYPE not set or invalid")
}

// NewRegistrator returns a new Registrator based on the DB_TYPE environment variable.
func NewRegistrator() (Registrator, error) {
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return &LocalRegistrator{}, nil
	} else if dbType == storage.DBTypeRemote {
		return &SupabaseRegistrator{}, nil
	}
	return nil, errors.New("DB_TYPE not set or invalid")
}

// AuthHandler provides handlers for the authentication routes of the application.
// It is responsible for handling user login, registration, and logout.
type AuthHandler struct {
	auth     Authenticator
	google   Authenticator
	deauth   Deauthenticator
	register Registrator
	verify   Verifier
}

// NewAuthHandler creates a new AuthHandler with the given LoginService and RegisterService, depending on the database type.
func NewAuthHandler() *AuthHandler {
	auth, _ := NewAuthenticator()
	google := &GoogleAuthenticator{}
	deauth := &LocalDeauthenticator{}
	register, _ := NewRegistrator()
	verify := &SupabaseVerifier{}
	return &AuthHandler{auth: auth, google: google, deauth: deauth, register: register, verify: verify}
}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
func (h AuthHandler) HandleGetLogin(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandleGetLogin()")
	return render(c, auth.Login())
}

// HandlePostLogin responds to POST on the /login route by trying to log in the user.
// If the user exists and the password is correct, the JWT tokens are generated and set as cookies.
// Finally, the user is redirected to the dashboard.
func (h AuthHandler) HandlePostLogin(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandlePostLogin()")
	return h.auth.Login(c)
}

// HandleGetLoginWithGoogle responds to GET on the /login/provider/google route by logging in the user with Google.
func (h AuthHandler) HandleGetLoginWithGoogle(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandleGetLoginWithGoogle()")
	return h.google.Login(c)
}

// HandlePostLogout responds to POST on the /logout route by logging out the user.
func (h AuthHandler) HandlePostLogout(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandlePostLogout()")
	return h.deauth.Logout(c)
}

// HandleGetRegister responds to GET on the /register route by rendering the Register component.
func (h AuthHandler) HandleGetRegister(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandleGetRegister()")
	return render(c, auth.Register())
}

// HandlePostRegister responds to POST on the /register route by trying to register the user.
// If the user does not exist, the password is hashed and the user is created in the database.
// Afterwards, the JWT tokens are generated and set as cookies. Finally, the user is redirected to the dashboard.
func (h AuthHandler) HandlePostRegister(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandlePostRegister()")
	return h.register.Register(c)
}

// HandleGetAuthCallback responds to GET on the /auth/callback route by verifying the user.
func (h AuthHandler) HandleGetAuthCallback(c echo.Context) error {
	slog.Info("💬 🔒 (pkg/handler/auth.go) HandleGetAuthCallback()")
	return h.verify.Verify(c)
}

// getAuthenticatedUser provides a shorthand function to get the authenticated user from the echo.Context.
func getAuthenticatedUser(c echo.Context) types.AuthenticatedUser {
	var user types.AuthenticatedUser
	slog.Info("💬 🔒 (pkg/handler/auth.go) getAuthenticatedUser()", "path", c.Request().URL.Path)
	u := c.Get(types.UserContextKey)
	if u == nil {
		slog.Debug("🚨 🔒 (pkg/handler/auth.go) ❓❓❓❓ 🥷 No user data found in echo.Context, trying with Cookie. Looked for", "contextKey", types.UserContextKey)
		cookie, err := c.Cookie(types.UserContextKey)
		if err != nil {
			slog.Info("✅ 🔒 (pkg/handler/auth.go) ❓❓❓❓ 🥷 No user cookie found, returning empty user. Looked for", "cookieName", types.UserContextKey)
			return types.AuthenticatedUser{}
		}
		slog.Info("✅ 🔒 (pkg/handler/auth.go) getAuthenticatedUser() -> 💃 User cookie found with", "name", types.UserContextKey, "value", cookie.Value)
		return types.AuthenticatedUser{
			Email:    cookie.Value,
			LoggedIn: true,
		}
	}
	user = u.(types.AuthenticatedUser)
	slog.Info("✅ 🔒 (pkg/handler/auth.go) getAuthenticatedUser() -> 💃 User data found in echo.Context with", "contextKey", types.UserContextKey, "email", user.Email, "loggedIn", user.LoggedIn)
	return user
}
