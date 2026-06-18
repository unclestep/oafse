package main

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/fx"

	"oafse/cmd/helpers"
	"oafse/internal/di"
	"oafse/internal/infrastructure/storage/postgres"

	_ "oafse/docs"
)

// @title           oafse Search API
// @version         1.0
// @description     Semantic search engine over crawled web pages.
// @host            localhost:14444
// @BasePath        /
// @produce         json
func main() {
	_ = godotenv.Load(".env")
	helpers.FallbackEnv("POSTGRES_DSN", "POSTGRES_LOCAL_DSN")
	helpers.FallbackEnv("REDIS_DSN", "REDIS_LOCAL_DSN")
	helpers.FallbackEnv("EMBEDDER_URL", "EMBEDDER_LOCAL_URL")
	helpers.FallbackEnv("SERVER_ADDR", "SERVER_LOCAL_URL")

	if err := postgres.Migrate(os.Getenv("POSTGRES_DSN")); err != nil {
		log.Fatalf("migration failed: %s", err)
	}

	fx.New(di.NewServer()).Run()
}
