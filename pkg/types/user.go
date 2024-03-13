package types

import (
	"time"

	"github.com/google/uuid"
	"github.com/uptrace/bun"
)

// UserContextKey is the key used to store the user in the context.
const UserContextKey = "wits-user"

// UserIdKey is the key used to store the user id in the context.
const UserIdKey = "wits-user-id"

// AuthenticatedUser represents the wrapper for an authenticated user and their logged-in state, as well as embedding the account.
type AuthenticatedUser struct {
	bun.BaseModel `bun:"auth.users,alias:u"`
	ID            uuid.UUID `bun:"type:uuid,default:uuid_generate_v4()"`
	Email         string
	Password      string
	LoggedIn      bool      `bun:"-"`
	CreatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt     time.Time `bun:",nullzero,notnull,default:current_timestamp"`

	Account Account
}
