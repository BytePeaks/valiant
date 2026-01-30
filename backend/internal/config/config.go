package config

import "os"

type Config struct {
	DatabaseURL string
	Port        string
}

func Load() *Config {
	return &Config{
		DatabaseURL: GetEnv("DATABASE_URL", "postgres://user:password@localhost:5432/valiant?sslmode=disable"),
		Port:        GetEnv("PORT", "8080"),
	}
}

func GetEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
