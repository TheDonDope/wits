package view

import (
	"context"
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
)

// AuthenticatedUser returns the authenticated user from the context.
func AuthenticatedUser(ctx context.Context) types.AuthenticatedUser {
	slog.Info("💬 🔮 (pkg/view/views.go) AuthenticatedUser()")
	var authenticatedUser types.AuthenticatedUser
	userContext := ctx.Value(types.UserContextKey)
	if userContext == nil {
		slog.Debug("✅ 🔮 (pkg/view/views.go) 🥷 No User data found in context.Context, returning empty user. Looked for", "contextKey", types.UserContextKey)
		return types.AuthenticatedUser{}
	}
	authenticatedUser = userContext.(types.AuthenticatedUser)
	slog.Info("✅ 🔮 (pkg/view/views.go) AuthenticatedUser() -> 💃 User data found in context.Context with", "email", authenticatedUser.Email, "loggedIn", authenticatedUser.LoggedIn)
	return authenticatedUser
}
