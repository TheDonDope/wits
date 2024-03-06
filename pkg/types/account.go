package types

import (
	"time"

	"github.com/google/uuid"
)

// Account is the type for the account of an authenticated user.
type Account struct {
	ID        uuid.UUID `bun:"type:uuid,default:uuid_generate_v4()"`
	UserID    uuid.UUID
	Username  string
	CreatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
	UpdatedAt time.Time `bun:",nullzero,notnull,default:current_timestamp"`
}
