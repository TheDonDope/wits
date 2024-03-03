package types

import "gorm.io/gorm"

// UserContextKey is the key used to store the user in the context.
const UserContextKey = "witsUser"

// User represents a user in the system.
type User struct {
	gorm.Model
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
	LoggedIn bool   `json:"-"`
}
