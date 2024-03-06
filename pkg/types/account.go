package types

import (
	"time"

	"github.com/google/uuid"
)

// Account is the type for the account of an authenticated user.
type Account struct {
	ID        int `bun:"id,pk,autoicrement"`
	UserID    uuid.UUID
	Username  string
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}
