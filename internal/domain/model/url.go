package model

import (
	"fmt"
	"net/url"
	"time"
)

type URL struct {
	*url.URL

	Try     int
	RetryAt time.Time
}

type CrawlStatus int

const (
	QueuedCrawlStatus CrawlStatus = iota
	ProcessingCrawlStatus
	DoneCrawlStatus
	GiveUpCrawlStatus
	RetryCrawlStatus
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
	case GiveUpCrawlStatus:
		return "GiveUP"
	default:
		return "Unknown"
	}
}

func NewURLFromParsed(u *url.URL) (*URL, error) {
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("new url from parsed: not an absolute URL: %s", u.String())
	}

	norm := Normalize(u)

	return &URL{
		URL: norm,
	}, nil
}

func NewURL(raw string) (*URL, error) {
	unnorm, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("new url: %w", err)
	}

	norm := Normalize(unnorm)

	return &URL{
		URL: norm,
	}, nil
}

func Normalize(unnorm *url.URL) *url.URL {
	norm := *unnorm
	norm.User = nil
	norm.Fragment = ""
	norm.RawFragment = ""
	norm.RawQuery = ""
	norm.ForceQuery = false
	if norm.Path == "/" {
		norm.Path = ""
	}
	return &norm
}

func (u *URL) IsSameDomain(other *URL) bool {
	return u.Hostname() == other.Hostname()
}

func (u *URL) Equals(other *URL) bool {
	return u.String() == other.String()
}
