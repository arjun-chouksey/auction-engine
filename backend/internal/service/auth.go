package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
	"golang.org/x/crypto/bcrypt"
)

type UserReader interface {
	FindByEmail(ctx context.Context, email string) (domain.User, error)
	FindByID(ctx context.Context, id string) (domain.User, error)
}

type Auth struct {
	users  UserReader
	tokens *auth.TokenManager
}

type LoginResult struct {
	AccessToken     string
	ExpiresInSecond int64
	User            domain.User
}

func NewAuth(users UserReader, tokens *auth.TokenManager) *Auth {
	return &Auth{users: users, tokens: tokens}
}

func (service *Auth) Login(ctx context.Context, email, password string) (LoginResult, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return LoginResult{}, ErrInvalidCredentials
	}
	user, err := service.users.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, postgresrepository.ErrNotFound) {
			return LoginResult{}, ErrInvalidCredentials
		}
		return LoginResult{}, fmt.Errorf("find login user: %w", err)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return LoginResult{}, ErrInvalidCredentials
	}
	token, err := service.tokens.Issue(user)
	if err != nil {
		return LoginResult{}, fmt.Errorf("issue access token: %w", err)
	}
	return LoginResult{AccessToken: token, ExpiresInSecond: service.tokens.ExpiresInSeconds(), User: user}, nil
}

func (service *Auth) Me(ctx context.Context, userID string) (domain.User, error) {
	user, err := service.users.FindByID(ctx, userID)
	if err != nil {
		if errors.Is(err, postgresrepository.ErrNotFound) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("find current user: %w", err)
	}
	return user, nil
}
