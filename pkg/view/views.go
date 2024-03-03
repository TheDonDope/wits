package view

import (
	"context"
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
)

// AuthenticatedUser returns the authenticated user from the context.
func AuthenticatedUser(ctx context.Context) types.User {
	var user types.User
	slog.Info("💬 🤝 (pkg/view/views.go) AuthenticatedUser")
	u := ctx.Value(types.UserContextKey)
	if u == nil {
		slog.Info("✅ 🤝 (pkg/view/views.go) 📦 No User data found in context.Context, returning empty user")
		return types.User{
			Email:    "anon@foo.org",
			LoggedIn: true,
		}
	}
	user = u.(types.User)
	slog.Info("✅ 🤝 (pkg/view/views.go) 📦 User data found in context.Context with", "email", user.Email, "loggedIn", user.LoggedIn)
	return user
}
