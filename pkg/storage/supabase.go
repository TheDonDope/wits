package storage

import (
	"log/slog"
	"os"

	"github.com/nedpals/supabase-go"
)

// DBTypeRemote is the variant of using a remote supabase database
const DBTypeRemote = "remote"

// SupabaseClient is the supabase client for the application
var SupabaseClient *supabase.Client

// InitSupabaseDB initializes the supabase client.
func InitSupabaseDB() error {
	slog.Info("💬 🛰️  (pkg/storage/supabase.go) InitSupabaseDB()")
	sbURL := os.Getenv("SUPABASE_URL")
	sbSecret := os.Getenv("SUPABASE_SECRET")
	SupabaseClient = supabase.CreateClient(sbURL, sbSecret)
	slog.Info("✅ 🛰️  (pkg/storage/supabase.go) 📂 Using Supabase client with", "url", sbURL)
	return nil
}
