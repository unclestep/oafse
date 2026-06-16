package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/fx"

	"oafse/internal/di"
	"oafse/internal/infrastructure/storage/postgres"
)

func main() {
	_ = godotenv.Load(".env")
	fallbackEnv("POSTGRES_DSN", "POSTGRES_LOCAL_DSN")
	fallbackEnv("REDIS_DSN", "REDIS_LOCAL_DSN")
	fallbackEnv("EMBEDDER_DSN", "EMBEDDER_LOCAL_DSN")

	if err := postgres.Migrate(os.Getenv("POSTGRES_DSN")); err != nil {
		log.Fatalf("migration failed: %s", err)
	}

	fx.New(di.NewIndexer()).Run()
}

func fallbackEnv(key, fallbackKey string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, os.Getenv(fallbackKey))
	}
}
