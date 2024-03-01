package storage

import (
	"log/slog"
	"os"

	"github.com/nedpals/supabase-go"
)

// SupabaseClient is the supabase client for the application
var SupabaseClient *supabase.Client

// InitSupabaseClient initializes the supabase client.
func InitSupabaseClient() error {
	sbURL := os.Getenv("SUPABASE_URL")
	sbSecret := os.Getenv("SUPABASE_SECRET")
	SupabaseClient = supabase.CreateClient(sbURL, sbSecret)
	slog.Info("📁 🛰️  Using Supabase with", "url", sbURL)
	return nil
}
