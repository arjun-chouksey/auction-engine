package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	Port string
}

func Load() (Config, error) {
	port := strings.TrimSpace(os.Getenv("APP_PORT"))
	if port == "" {
		port = "8080"
	}
	if strings.ContainsAny(port, ":/ ") {
		return Config{}, fmt.Errorf("APP_PORT must be a port number")
	}
	return Config{Port: port}, nil
}
