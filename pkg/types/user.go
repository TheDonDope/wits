package types

import "gorm.io/gorm"

// User represents a user in the system.
type User struct {
	gorm.Model
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}
