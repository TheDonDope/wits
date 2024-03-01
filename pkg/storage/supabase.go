package storage

import (
	"log/slog"
	"os"

	"github.com/nedpals/supabase-go"
)

// Client is the supabase client for the application
var Client *supabase.Client

// InitSupabaseClient initializes the supabase client.
func InitSupabaseClient() error {
	sbURL := os.Getenv("SUPABASE_URL")
	sbSecret := os.Getenv("SUPABASE_SECRET")
	Client = supabase.CreateClient(sbURL, sbSecret)
	slog.Info("📁 🛰️  Using Supabase with", "url", sbURL)
	return nil
}
