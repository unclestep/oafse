package main

import (
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"go.uber.org/fx"

	"oafse/internal/di"
	"oafse/internal/infrastructure/storage/postgres"
)

func main() {
	startURL := flag.String("u", "https://quotes.toscrape.com", "Start URL for crawling")
	resume := flag.Bool("resume", false, "Resume crawling")
	flag.Parse()

	_ = godotenv.Load(".env")
	fallbackEnv("POSTGRES_DSN", "POSTGRES_LOCAL_DSN")
	fallbackEnv("REDIS_DSN", "REDIS_LOCAL_DSN")

	if err := postgres.Migrate(os.Getenv("POSTGRES_DSN")); err != nil {
		log.Fatalf("migration failed: %s", err)
	}

	baseURL, err := normalizeStartURL(*startURL)
	if err != nil {
		log.Fatalf("invalid start URL: %s", err)
	}

	fx.New(di.NewCrawler(baseURL, *resume)).Run()
}

func fallbackEnv(key, fallbackKey string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, os.Getenv(fallbackKey))
	}
}

func normalizeStartURL(raw string) (string, error) {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("missing host in %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}
