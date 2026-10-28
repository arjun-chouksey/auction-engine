package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("not found")

type Users struct {
	pool *pgxpool.Pool
}

func NewUsers(pool *pgxpool.Pool) *Users {
	return &Users{pool: pool}
}

func (repository *Users) FindByEmail(ctx context.Context, email string) (domain.User, error) {
	return repository.scanUser(repository.pool.QueryRow(ctx, `
		SELECT id, full_name, email, password_hash, role
		FROM users
		WHERE lower(email) = lower($1)
	`, email))
}

func (repository *Users) FindByID(ctx context.Context, id string) (domain.User, error) {
	return repository.scanUser(repository.pool.QueryRow(ctx, `
		SELECT id, full_name, email, password_hash, role
		FROM users
		WHERE id = $1
	`, id))
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (repository *Users) scanUser(row rowScanner) (domain.User, error) {
	var user domain.User
	if err := row.Scan(&user.ID, &user.FullName, &user.Email, &user.PasswordHash, &user.Role); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("scan user: %w", err)
	}
	return user, nil
}
