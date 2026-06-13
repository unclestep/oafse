package di

import (
	"net/http"
	"time"

	"go.uber.org/fx"
)

var fetcher = fx.Module(
	"fetcher",
	fx.Provide(func() *http.Client {
		return &http.Client{
			Timeout: 15 * time.Second,
		}
	}),
)
