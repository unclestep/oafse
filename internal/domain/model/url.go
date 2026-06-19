package model

import (
	"fmt"
	"net/url"
	"strings"
)

type URL struct {
	*url.URL

	Parent string
	Try    int
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

func (p CrawlStatus) String() string {
	switch p {
	case QueuedCrawlStatus:
		return "Queued"
	case ProcessingCrawlStatus:
		return "Processing"
	case DoneCrawlStatus:
		return "Done"
	case ManualCrawlStatus:
		return "Manual"
	case RetryCrawlStatus:
		return "Retry"
	case GiveUpCrawlStatus:
		return "GiveUp"
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
	if unnorm.Scheme == "" || unnorm.Host == "" {
		return nil, fmt.Errorf("new url: not an absolute URL: %s", unnorm.String())
	}

	norm := Normalize(unnorm)

	return &URL{
		URL: norm,
	}, nil
}

func Normalize(unnorm *url.URL) *url.URL {
	norm := *unnorm

	norm.Host = strings.ToLower(norm.Host)
	norm.User = nil
	norm.Fragment = ""
	norm.RawFragment = ""
	norm.RawQuery = ""
	norm.ForceQuery = false
	norm.Path = strings.TrimRight(norm.Path, "/")

	return &norm
}

func (u *URL) IsSameDomain(other *URL) bool {
	return u.Hostname() == other.Hostname()
}

func (u *URL) Equals(other *URL) bool {
	return u.String() == other.String()
}

func NormalizeStartURL(raw string) (string, error) {
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("normalize start url: %w", err)
	}
	if u.Host == "" {
		return "", fmt.Errorf("normalize start url: missing host in %q", raw)
	}
	return u.Scheme + "://" + u.Host, nil
}
