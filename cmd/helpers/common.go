package helpers

import (
	"os"
)

func FallbackEnv(key, fallbackKey string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, os.Getenv(fallbackKey))
	}
}
