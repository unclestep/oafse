package model

import "time"

type CrawlConfig struct {
	StartURL                  string
	WorkersCount              int
	TryLim                    int
	TryBaseInterval           time.Duration
	HealthCheckWorryThreshold time.Duration
}
