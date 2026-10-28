package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/arjunchouksey/british-auction-rfq/backend/internal/auth"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/config"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/database"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/httpapi"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/realtime"
	postgresrepository "github.com/arjunchouksey/british-auction-rfq/backend/internal/repository/postgres"
	"github.com/arjunchouksey/british-auction-rfq/backend/internal/service"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "check the local health endpoint")
	flag.Parse()

	if *healthcheck {
		if err := checkHealth(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	pool, err := database.Open(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	tokens, err := auth.NewTokenManager(cfg.JWTSecret, time.Hour)
	if err != nil {
		slog.Error("configure token manager", "error", err)
		os.Exit(1)
	}
	users := postgresrepository.NewUsers(pool)
	rfqs := postgresrepository.NewRFQs(pool)
	bids := postgresrepository.NewBids(pool)
	events := postgresrepository.NewEvents(pool)
	hub := realtime.NewHub(4)
	authService := service.NewAuth(users, tokens)
	rfqService := service.NewRFQs(rfqs)
	bidService := service.NewBids(bids, hub)
	eventService := service.NewEvents(events)

	server := &http.Server{
		Addr: ":" + cfg.Port,
		Handler: httpapi.NewRouter(httpapi.Dependencies{
			AuthService:  authService,
			RFQService:   rfqService,
			BidService:   bidService,
			EventService: eventService,
			Hub:          hub,
			Tokens:       tokens,
		}),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	shutdownContext, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-shutdownContext.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			slog.Error("graceful shutdown", "error", err)
		}
	}()

	slog.Info("API listening", "address", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("serve API", "error", err)
		os.Exit(1)
	}
}

func checkHealth() error {
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "8080"
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Get("http://127.0.0.1:" + port + "/healthz")
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("health request returned %s", response.Status)
	}
	return nil
}
