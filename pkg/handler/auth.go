package handler

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/TheDonDope/wits/pkg/types"
	"github.com/TheDonDope/wits/pkg/view/auth"
	"github.com/nedpals/supabase-go"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/labstack/echo/v4"
)

// AuthHandler provides handlers for the authentication routes of the application.
// It is responsible for handling user login, registration, and logout.
type AuthHandler struct{}

// HandleGetLogin responds to GET on the /login route by rendering the Login component.
func (h AuthHandler) HandleGetLogin(c echo.Context) error {
	return render(c, auth.Login())
}

// HandlePostLogin responds to POST on the /login route by trying to log in the user.
// If the user exists and the password is correct, the JWT tokens are generated and set as cookies.
// Finally, the user is redirected to the dashboard.
func (h AuthHandler) HandlePostLogin(c echo.Context) error {
	slog.Info("🔐 🤝 Logging in user")
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return loginLocal(c)
	} else if dbType == storage.DBTypeRemote {
		return loginRemote(c)
	}
	return nil
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
	dbType := os.Getenv("DB_TYPE")
	if dbType == storage.DBTypeLocal {
		return registerLocal(c)
	} else if dbType == storage.DBTypeRemote {
		return registerRemote(c)
	}
	return nil
}

// loginLocal logs in the user with the local sqlite database.
func loginLocal(c echo.Context) error {
	slog.Info("🔐 🏠 Logging in user with local sqlite database")
	user, userErr := readByEmailAndPassword(c.FormValue("email"), c.FormValue("password"))
	if userErr != nil {
		slog.Error("🚨 🤝 Checking if user exists failed with", "error", userErr)
		return echo.NewHTTPError(http.StatusNotFound, "User not found")
	}

	// Generate JWT tokens and set cookies 'manually'
	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("🔀 🤝 Redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}

// loginRemote logs in the user with the remote Supabase database.
func loginRemote(c echo.Context) error {
	slog.Info("🔐 🛰️  Logging in user with remote Supabase database")
	credentials := supabase.UserCredentials{
		Email:    c.FormValue("email"),
		Password: c.FormValue("password"),
	}

	// Call Supabase to sign in
	signInResp, err := storage.SupabaseClient.Auth.SignIn(c.Request().Context(), credentials)
	if err != nil {
		slog.Error("🚨 🤝 Signing user in with Supabase failed with", "error", err)
		return render(c, auth.LoginForm(credentials, auth.LoginErrors{
			InvalidCredentials: "The credentials you have entered are invalid",
		}))
	}
	slog.Info("✅ 🤝 User has been logged in with", "signInResp", signInResp)

	// Checkme:
	c.SetCookie(&http.Cookie{
		Value:    signInResp.AccessToken,
		Name:     "at",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
	})

	user := &types.User{
		Email: signInResp.User.Email,
		Name:  signInResp.User.ID,
	}

	// Generate JWT tokens and set cookies 'manually'
	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("🔀 🤝 Redirecting to dashboard")
	return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}

// registerLocal registers the user with the local sqlite database.
func registerLocal(c echo.Context) error {
	slog.Info("🔐 🏠 Registering user with local sqlite database")
	params := auth.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🤝 Passwords do not match")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}

	// Check if user with email already exists
	existingUser, err := readByEmail(params.Email)
	if err != nil {
		slog.Error("🚨 🤝 Checking if user exists failed with", "error", err)
	}

	if existingUser != nil {
		slog.Error("🚨 🤝 User with email already exists")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "User with email already exists",
		}))
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(params.Password), 8)
	if err != nil {
		slog.Error("🚨 🤝 Hashing password failed with", "error", err)
	}

	user := &types.User{
		Email:    params.Email,
		Password: string(hashedPassword),
		Name:     params.Username,
	}

	storage.SQLiteDB.Create(&user)

	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("✅ 🤝 User has been registered, redirecting to dashboard (reactivate me maybe lol)")
	return render(c, auth.RegisterSuccess(params.Email))
	//return c.Redirect(http.StatusMovedPermanently, "/dashboard")
}

// registerRemote registers the user with the remote Supabase database.
func registerRemote(c echo.Context) error {
	slog.Info("🔐 🛰️  Registering user with remote Supabase database")
	params := auth.RegisterParams{
		Username:             c.FormValue("username"),
		Email:                c.FormValue("email"),
		Password:             c.FormValue("password"),
		PasswordConfirmation: c.FormValue("password-confirmation"),
	}

	if params.Password != params.PasswordConfirmation {
		slog.Error("🚨 🤝 Passwords do not match")
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: "The passwords do not match",
		}))
	}
	// Call Supabase to sign up
	signUpResp, err := storage.SupabaseClient.Auth.SignUp(c.Request().Context(), supabase.UserCredentials{Email: params.Email, Password: params.Password})
	if err != nil {
		slog.Error("🚨 🤝 Signing user up with Supabase failed with", "error", err)
		return render(c, auth.RegisterForm(params, auth.RegisterErrors{
			InvalidCredentials: err.Error(),
		}))
	}
	slog.Info("✅ 🤝 User has been signed up with Supabase with", "signUpResp", signUpResp)

	user := &types.User{
		Email: params.Email,
		Name:  params.Username,
	}

	tokenErr := GenerateTokensAndSetCookies(user, c)
	if tokenErr != nil {
		slog.Error("🚨 🤝 Generating tokens failed with", "error", tokenErr)
		return echo.NewHTTPError(http.StatusUnauthorized, "Token is incorrect")
	}

	slog.Info("✅ 🤝 User has been registered, redirecting to dashboard (reactivate me maybe lol)")
	return render(c, auth.RegisterSuccess(params.Email))
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
