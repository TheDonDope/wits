package handler

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/TheDonDope/wits/pkg/storage"
	"github.com/labstack/echo/v4"
	"github.com/nedpals/supabase-go"
)

// GoogleAuthenticator is an interface for the user login, when using Google.
type GoogleAuthenticator struct{}

// Login logs in the user with their Google Credentials
func (g GoogleAuthenticator) Login(c echo.Context) error {
	slog.Info("💬 🛰️  (pkg/handler/auth_google.go) GoogleAuthenticator.Login()")
	resp, err := storage.SupabaseClient.Auth.SignInWithProvider(supabase.ProviderSignInOptions{
		Provider:   "google",
		RedirectTo: os.Getenv("AUTH_CALLBACK_URL"),
	})
	if err != nil {
		return err
	}
	slog.Info("🆗 🛰️  (pkg/handler/auth_google.go)  🔓 User has been logged in with Google")
	slog.Info("✅ 🛰️  (pkg/handler/auth_google.go) RemoteAuthenticator.Login() -> 🔀 Redirecting to", "url", resp.URL[:10]+"...")
	return c.Redirect(http.StatusSeeOther, resp.URL)
}
