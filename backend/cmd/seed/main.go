package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/database"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	password := os.Getenv("DEMO_PASSWORD")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := database.Open(ctx, databaseURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := postgresrepository.SeedDevelopmentUsers(ctx, pool, password); err != nil {
		slog.Error("seed development users", "error", err)
		os.Exit(1)
	}
	slog.Info("development users are present", "count", len(postgresrepository.DevelopmentUsers()))
}
