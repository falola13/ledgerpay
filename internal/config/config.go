package config

import "os"

type Config struct {
	Port           string
	JWTSecret      string
	DatabaseURL    string
	RedisAddr      string
	WEBHOOK_SECRET string
	WEBHOOK_URL    string
}

func Load() Config {
	return Config{
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		Port:           gotenv("PORT", "8080"),
		JWTSecret:      os.Getenv("JWTSecret"),
		RedisAddr:      os.Getenv("RedisAddr"),
		WEBHOOK_URL:    os.Getenv("WEBHOOK_URL"),
		WEBHOOK_SECRET: os.Getenv("WEBHOOK_SECRET"),
	}
}

func gotenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
