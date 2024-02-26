package storage

import (
	"database/sql"
	"fmt"
	"os"

	"github.com/TheDonDope/wits/pkg/types"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserStorage provides all storage related functionality for users.
type UserStorage struct {
	DB *sql.DB
}

// GetTestUserByEmailAndPassword returns a test user by email and password.
func (s *UserStorage) GetTestUserByEmailAndPassword(email string, password string) (*types.User, error) {
	testUsers, err := s.GetTestUsers()
	if err != nil {
		return nil, err
	}

	for _, user := range testUsers {
		if user.Email == email && bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) == nil {
			return user, nil
		}
	}

	return nil, fmt.Errorf("User not found")
}

// GetTestUsers returns a list of test users.
func (s *UserStorage) GetTestUsers() ([]*types.User, error) {
	onePasswd, err := bcrypt.GenerateFromPassword([]byte(os.Getenv("ONE_PASSWORD")), 8)
	if err != nil {
		return nil, err
	}
	one := &types.User{
		ID:       uuid.NewString(),
		Email:    os.Getenv("ONE_EMAIL"),
		Password: string(onePasswd),
	}
	twoPasswd, err := bcrypt.GenerateFromPassword([]byte(os.Getenv("TWO_PASSWORD")), 8)
	if err != nil {
		return nil, err
	}
	two := &types.User{
		ID:       uuid.NewString(),
		Email:    os.Getenv("TWO_EMAIL"),
		Password: string(twoPasswd),
	}
	return []*types.User{one, two}, nil
}
