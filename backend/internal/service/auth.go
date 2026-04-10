package service

import (
	"context"

	"github.com/yubrajnag/taskflow/backend/internal/auth"
	"github.com/yubrajnag/taskflow/backend/internal/domain"
	"github.com/yubrajnag/taskflow/backend/internal/repository"
)

type AuthService struct {
	users      repository.UserRepository
	tokens     *auth.TokenService
	bcryptCost int
}

func NewAuthService(users repository.UserRepository, tokens *auth.TokenService, bcryptCost int) *AuthService {
	return &AuthService{
		users:      users,
		tokens:     tokens,
		bcryptCost: bcryptCost,
	}
}

func (s *AuthService) Register(ctx context.Context, name, email, password string) (string, error) {
	user, err := domain.NewUser(name, email, password, s.bcryptCost)
	if err != nil {
		return "", err
	}

	if err := s.users.Create(ctx, user); err != nil {
		return "", err
	}

	return s.tokens.Generate(user.ID, user.Email)
}

// Both "not found" and "wrong password" return ErrUnauthorized to prevent user enumeration.
func (s *AuthService) Login(ctx context.Context, email, password string) (string, error) {
	user, err := s.users.GetByEmail(ctx, email)
	if err != nil {
		if err == domain.ErrNotFound {
			return "", domain.ErrUnauthorized
		}
		return "", err
	}

	if !user.CheckPassword(password) {
		return "", domain.ErrUnauthorized
	}

	return s.tokens.Generate(user.ID, user.Email)
}
