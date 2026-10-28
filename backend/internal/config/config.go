package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
}

func Load() (Config, error) {
	port := strings.TrimSpace(os.Getenv("APP_PORT"))
	if port == "" {
		port = "8080"
	}
	if strings.ContainsAny(port, ":/ ") {
		return Config{}, fmt.Errorf("APP_PORT must be a port number")
	}
	databaseURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if databaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	return Config{Port: port, DatabaseURL: databaseURL}, nil
}
