package main

import (
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/fx"

	"oafse/cmd/helpers"
	"oafse/internal/di"
	"oafse/internal/infrastructure/storage/postgres"
)

func main() {
	startURL := flag.String("u", "https://quotes.toscrape.com", "Start URL for crawling")
	resume := flag.Bool("resume", false, "Resume crawling")
	flag.Parse()

	_ = godotenv.Load(".env")
	helpers.FallbackEnv("POSTGRES_DSN", "POSTGRES_LOCAL_DSN")
	helpers.FallbackEnv("REDIS_DSN", "REDIS_LOCAL_DSN")

	if err := postgres.Migrate(os.Getenv("POSTGRES_DSN")); err != nil {
		log.Fatalf("migration failed: %s", err)
	}

	baseURL, err := helpers.NormalizeStartURL(*startURL)
	if err != nil {
		log.Fatalf("invalid start URL: %s", err)
	}

	fx.New(di.NewCrawler(baseURL, *resume)).Run()
}
