package helpers

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func FallbackEnv(key, fallbackKey string) {
	if os.Getenv(key) == "" {
		_ = os.Setenv(key, os.Getenv(fallbackKey))
	}
}

func NormalizeStartURL(raw string) (string, error) {
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
