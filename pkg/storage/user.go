package storage

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/TheDonDope/wits/pkg/types"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserStorage provides all storage related functionality for users.
type UserStorage struct {
	DB *gorm.DB
}

// GetUserByEmail returns a user with the given email.
func (s *UserStorage) GetUserByEmail(email string) (*types.User, error) {
	var user types.User
	err := s.DB.Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		slog.Error("🚨 📁 Finding user failed with", "error", err)
		return nil, err
	}

	return &user, nil
}

// GetUserByEmailAndPassword returns a user with the given email and password.
func (s *UserStorage) GetUserByEmailAndPassword(email string, password string) (*types.User, error) {
	user, err := s.GetUserByEmail(email)
	if err != nil {
		slog.Error("🚨 📁 Finding user by email failed with", "error", err)
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		slog.Error("🚨 📁 Password is incorrect")
		return nil, fmt.Errorf("Password is incorrect")
	}

	return user, nil
}
