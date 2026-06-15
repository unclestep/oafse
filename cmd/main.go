package main

import (
	"flag"
	"log"
	"os"

	"github.com/joho/godotenv"
	"go.uber.org/fx"

	"oafse/internal/di"
	"oafse/internal/infrastructure/storage/postgres"
)

func main() {
	startURL := flag.String("-u", "https://quotes.toscrape.com", "Domain to parse")
	_ = godotenv.Load(".env")
	if err := postgres.Migrate(os.Getenv("POSTGRES_DSN")); err != nil {
		log.Fatalf("migration failed: %s", err)
	}

	fx.New(di.Crawler).Run()
}
