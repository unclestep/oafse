package model

import "time"

type CrawlConfig struct {
	WorkersCount              int
	TryLim                    int
	TryBaseInterval           time.Duration
	HealthCheckWorryThreshold time.Duration
}
