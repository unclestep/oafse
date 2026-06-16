package model

import (
	"time"
)

type URLCache struct {
	URL    string
	Parent string // Metadata is not required because it is already processed
	Try    int
}

type CrawlMetadata struct {
	UnprocessedCount int
	EarliestRetry    time.Time
}

type CrawlStatus int

const (
	UnknownCrawlStatus CrawlStatus = iota
	QueuedCrawlStatus
	ProcessingCrawlStatus
	DoneCrawlStatus
	ManualCrawlStatus
	GiveUpCrawlStatus
	RetryCrawlStatus
)
