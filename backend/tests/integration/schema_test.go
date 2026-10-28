//go:build integration

package integration_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/database"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const demoPassword = "integration-test-password"

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	databaseURL := os.Getenv("DATABASE_TEST_URL")
	if databaseURL == "" {
		fmt.Fprintln(os.Stderr, "DATABASE_TEST_URL is required for integration tests")
		os.Exit(1)
	}
	if err := requireTestDatabase(databaseURL); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	migrator, err := newMigrator(databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := migrator.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		fmt.Fprintln(os.Stderr, "reset test schema:", err)
		os.Exit(1)
	}
	if err := migrator.Up(); err != nil {
		fmt.Fprintln(os.Stderr, "apply test migrations:", err)
		os.Exit(1)
	}
	sourceErr, databaseErr := migrator.Close()
	if sourceErr != nil || databaseErr != nil {
		fmt.Fprintln(os.Stderr, "close migrator:", sourceErr, databaseErr)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	testPool, err = database.Open(ctx, databaseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := postgresrepository.SeedDevelopmentUsers(ctx, testPool, demoPassword); err != nil {
		fmt.Fprintln(os.Stderr, err)
		testPool.Close()
		os.Exit(1)
	}

	code := m.Run()
	testPool.Close()
	os.Exit(code)
}

func TestMigrationRerunReportsNoChange(t *testing.T) {
	migrator, err := newMigrator(os.Getenv("DATABASE_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer migrator.Close()

	if err := migrator.Up(); !errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("second migration Up() error = %v, want migrate.ErrNoChange", err)
	}

	var tableCount int
	err = testPool.QueryRow(context.Background(), `
		SELECT count(*)
		FROM information_schema.tables
		WHERE table_schema = 'public'
		  AND table_name IN ('users', 'rfqs', 'bids', 'auction_events')
	`).Scan(&tableCount)
	if err != nil {
		t.Fatalf("count schema tables: %v", err)
	}
	if tableCount != 4 {
		t.Fatalf("schema table count = %d, want 4", tableCount)
	}
}

func TestDevelopmentSeedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	var originalHash string
	if err := testPool.QueryRow(ctx, `SELECT password_hash FROM users WHERE email = 'buyer@example.test'`).Scan(&originalHash); err != nil {
		t.Fatalf("read original buyer hash: %v", err)
	}

	if err := postgresrepository.SeedDevelopmentUsers(ctx, testPool, demoPassword); err != nil {
		t.Fatalf("seed second time: %v", err)
	}

	var userCount, buyerCount, supplierCount int
	var currentHash string
	err := testPool.QueryRow(ctx, `
		SELECT count(*),
		       count(*) FILTER (WHERE role = 'BUYER'),
		       count(*) FILTER (WHERE role = 'SUPPLIER'),
		       max(password_hash) FILTER (WHERE email = 'buyer@example.test')
		FROM users
	`).Scan(&userCount, &buyerCount, &supplierCount, &currentHash)
	if err != nil {
		t.Fatalf("inspect seeded users: %v", err)
	}
	if userCount != 4 || buyerCount != 1 || supplierCount != 3 {
		t.Fatalf("seed counts = users:%d buyers:%d suppliers:%d, want 4/1/3", userCount, buyerCount, supplierCount)
	}
	if currentHash != originalHash {
		t.Fatal("idempotent seed changed the stored password hash")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(demoPassword)); err != nil {
		t.Fatalf("seeded password hash does not match demo password: %v", err)
	}
}

func TestSchemaConstraints(t *testing.T) {
	t.Run("email is unique case insensitively", func(t *testing.T) {
		transaction, err := testPool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer transaction.Rollback(context.Background())

		_, err = transaction.Exec(context.Background(), `
			INSERT INTO users (full_name, email, password_hash, role)
			VALUES ('Duplicate Buyer', 'BUYER@EXAMPLE.TEST', 'not-a-real-hash', 'BUYER')
		`)
		assertPostgresCode(t, err, "23505")
	})

	t.Run("forced close must follow initial close", func(t *testing.T) {
		transaction, err := testPool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer transaction.Rollback(context.Background())

		_, err = transaction.Exec(context.Background(), `
			INSERT INTO rfqs (
				reference_code, name, buyer_id, service_date,
				bid_start_at, initial_close_at, current_close_at, forced_close_at,
				trigger_window_minutes, extension_minutes, trigger_type
			)
			VALUES (
				'INVALID-TIME', 'Invalid time ordering', $1, '2030-01-02',
				'2030-01-01T09:00:00Z', '2030-01-01T10:00:00Z',
				'2030-01-01T10:00:00Z', '2030-01-01T10:00:00Z',
				10, 5, 'ANY_BID'
			)
		`, postgresrepository.DevelopmentUsers()[0].ID)
		assertPostgresCode(t, err, "23514")
	})

	t.Run("current close cannot exceed forced close", func(t *testing.T) {
		transaction, err := testPool.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		defer transaction.Rollback(context.Background())

		_, err = transaction.Exec(context.Background(), `
			INSERT INTO rfqs (
				reference_code, name, buyer_id, service_date,
				bid_start_at, initial_close_at, current_close_at, forced_close_at,
				trigger_window_minutes, extension_minutes, trigger_type
			)
			VALUES (
				'INVALID-CURRENT', 'Invalid current close', $1, '2030-01-02',
				'2030-01-01T09:00:00Z', '2030-01-01T10:00:00Z',
				'2030-01-01T12:00:00Z', '2030-01-01T11:00:00Z',
				10, 5, 'ANY_BID'
			)
		`, postgresrepository.DevelopmentUsers()[0].ID)
		assertPostgresCode(t, err, "23514")
	})
}

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("find integration test path")
	}
	migrationsPath := filepath.Join(filepath.Dir(currentFile), "..", "..", "migrations")
	migrator, err := migrate.New("file://"+migrationsPath, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create migrator: %w", err)
	}
	return migrator, nil
}

func requireTestDatabase(databaseURL string) error {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return fmt.Errorf("parse DATABASE_TEST_URL: %w", err)
	}
	if !strings.HasSuffix(config.ConnConfig.Database, "_test") {
		return fmt.Errorf("refusing integration tests: database %q does not end in _test", config.ConnConfig.Database)
	}
	return nil
}

func assertPostgresCode(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("database error = nil, want PostgreSQL code %s", want)
	}
	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		t.Fatalf("database error = %T %v, want *pgconn.PgError", err, err)
	}
	if postgresError.Code != want {
		t.Fatalf("PostgreSQL code = %s, want %s (%v)", postgresError.Code, want, err)
	}
}
