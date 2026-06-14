package model

import (
	"time"
)

type URLCache struct {
	URL     string
	Try     int
	RetryAt time.Time
}

type CrawlMetadata struct {
	UnprocessedCount int
	EarliestRetry    time.Time
}
