package config

import (
	"os"
)

type Config struct {
	ServerAddress string
	DatabaseURL   string
}

func Load() *Config {
	return &Config{
		ServerAddress: getEnv("SERVER_PORT", ":8080"),
		DatabaseURL:   getEnv("DATABASE_URL", "postgres://postgres:password@localhost:5432/txn_service?sslmode=disable"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
