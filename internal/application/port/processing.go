package port

import (
	"time"

	"oafse/internal/domain/model"
)

type CrawlConfig struct {
	WorkersCount int
	TryLim                    int
	TryBaseInterval           time.Duration
	HealthCheckWorryThreshold time.Duration
}

type Processing interface {
	DecideRetry(url *model.URL, conf *CrawlConfig) (bool, time.Time)
}
