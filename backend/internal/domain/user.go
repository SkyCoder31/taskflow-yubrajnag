package domain

import (
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        uuid.UUID
	Name      string
	Email     string
	Password  string
	CreatedAt time.Time
}

func NewUser(name, email, plainPassword string, bcryptCost int) (*User, error) {
	ve := NewValidationError()

	name = strings.TrimSpace(name)
	if name == "" {
		ve.Add("name", "is required")
	}

	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		ve.Add("email", "is required")
	} else if _, err := mail.ParseAddress(email); err != nil {
		ve.Add("email", "is not a valid email address")
	}

	if plainPassword == "" {
		ve.Add("password", "is required")
	} else if len(plainPassword) < 8 {
		ve.Add("password", "must be at least 8 characters")
	}

	if ve.HasErrors() {
		return nil, ve
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(plainPassword), bcryptCost)
	if err != nil {
		return nil, err
	}

	return &User{
		ID:        uuid.New(),
		Name:      name,
		Email:     email,
		Password:  string(hashed),
		CreatedAt: time.Now().UTC(),
	}, nil
}

func (u *User) CheckPassword(plainPassword string) bool {
	return bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(plainPassword)) == nil
}
