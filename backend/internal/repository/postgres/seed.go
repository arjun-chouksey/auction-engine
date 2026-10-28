package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type DemoUser struct {
	ID       string
	FullName string
	Email    string
	Role     string
}

var demoUsers = []DemoUser{
	{ID: "10000000-0000-4000-8000-000000000001", FullName: "Demo Buyer", Email: "buyer@example.test", Role: "BUYER"},
	{ID: "20000000-0000-4000-8000-000000000001", FullName: "Demo Supplier A", Email: "supplier.a@example.test", Role: "SUPPLIER"},
	{ID: "20000000-0000-4000-8000-000000000002", FullName: "Demo Supplier B", Email: "supplier.b@example.test", Role: "SUPPLIER"},
	{ID: "20000000-0000-4000-8000-000000000003", FullName: "Demo Supplier C", Email: "supplier.c@example.test", Role: "SUPPLIER"},
}

func SeedDevelopmentUsers(ctx context.Context, pool *pgxpool.Pool, password string) error {
	if password == "" {
		return fmt.Errorf("demo password is required")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash demo password: %w", err)
	}

	transaction, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	for _, user := range demoUsers {
		if _, err := transaction.Exec(ctx, `
			INSERT INTO users (id, full_name, email, password_hash, role)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT DO NOTHING
		`, user.ID, user.FullName, user.Email, string(hash), user.Role); err != nil {
			return fmt.Errorf("insert demo user %s: %w", user.Email, err)
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}

func DevelopmentUsers() []DemoUser {
	users := make([]DemoUser, len(demoUsers))
	copy(users, demoUsers)
	return users
}
