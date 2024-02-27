package storage

import (
	"fmt"

	"github.com/TheDonDope/wits/pkg/types"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// UserStorage provides all storage related functionality for users.
type UserStorage struct {
	DB *gorm.DB
}

// GetUserByEmailAndPassword returns a user with the given email and password.
func (s *UserStorage) GetUserByEmailAndPassword(email string, password string) (*types.User, error) {
	var user types.User
	row := s.DB.Where("email = ?", email).First(&user)
	if row.Error != nil {
		return nil, fmt.Errorf("User not found")

	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)) != nil {
		return nil, fmt.Errorf("Password is incorrect")
	}

	return &user, nil
}

// InsertTestUsers inserts some test users into the database.
func (s *UserStorage) InsertTestUsers() {
	onePasswd, err := bcrypt.GenerateFromPassword([]byte("known"), 8)
	if err != nil {
		fmt.Println("Error hashing password one")
	}
	one := &types.User{
		Email:    "one@foo.org",
		Password: string(onePasswd),
		Name:     "One",
	}
	s.DB.Create(&one)

	twoPasswd, err := bcrypt.GenerateFromPassword([]byte("known"), 8)
	if err != nil {
		fmt.Println("Error hashing password two")
	}
	two := &types.User{
		Email:    "two@foo.org",
		Password: string(twoPasswd),
		Name:     "Two",
	}
	s.DB.Create(&two)
}
