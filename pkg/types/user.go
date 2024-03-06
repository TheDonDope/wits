package types

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// UserContextKey is the key used to store the user in the context.
const UserContextKey = "wits-user"

// AuthenticatedUser represents the wrapper for an authenticated user and their logged-in state, as well as embedding the account.
type AuthenticatedUser struct {
	ID       uuid.UUID `bun:"pk,type:uuid,default:uuid_generate_v4()"`
	Email    string
	Password string
	LoggedIn bool `bun:"-"`

	Account Account `bun:"rel:belongs-to"` // Check if this is correct
}

// User represents a user in the system.
type User struct {
	gorm.Model
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
