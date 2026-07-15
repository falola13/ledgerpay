package config

import "os"

type Config struct {
	Port        string
	JWTSecret   string
	DatabaseURL string
}

func Load() Config {
	return Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Port:        gotenv("PORT", "8080"),
		JWTSecret:   os.Getenv("JWTSecret"),
	}
}

func gotenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
