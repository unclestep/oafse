package model

import (
	"fmt"
	"net/url"
	"time"
)

type URL struct {
	norm     string
	host     string
	Status   CrawlStatus
	Try      int
	TakeOnAt time.Time
	RetryAt  time.Time
}

type CrawlStatus int

const (
	QueuedCrawlStatus CrawlStatus = iota
	ProcessingCrawlStatus
	DoneCrawlStatus
	GiveUpCrawlStatus
	RetryCrawlStatus
	ManualCrawlStatus
)

func (p CrawlStatus) String() string {
	switch p {
	case QueuedCrawlStatus:
		return "Queued"
	case ProcessingCrawlStatus:
		return "Processing"
	case DoneCrawlStatus:
		return "Done"
	case RetryCrawlStatus:
		return "Retry"
	case ManualCrawlStatus:
		return "Manual"
	default:
		return "Unknown"
	}
}

func NewURL(raw string) *URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		panic(fmt.Sprintf("new url: %s", err))
	}

	parsed.User = nil
	parsed.Fragment = ""
	parsed.RawFragment = ""
	parsed.RawQuery = ""
	parsed.ForceQuery = false

	return &URL{
		norm: parsed.String(),
		host: parsed.Hostname(),
	}
}

func (u *URL) String() string {
	return u.norm
}

func (u *URL) IsSameDomain(other *URL) bool {
	return u.host == other.host
}

func (u *URL) Equals(other *URL) bool {
	return u.norm == other.norm
}
