package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
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
	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		return Config{}, fmt.Errorf("JWT_SECRET must contain at least 32 characters")
	}
	return Config{Port: port, DatabaseURL: databaseURL, JWTSecret: jwtSecret}, nil
}
