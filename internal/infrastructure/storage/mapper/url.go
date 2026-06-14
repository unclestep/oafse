package mapper

import (
	"net/url"

	"oafse/internal/application/port"
	domain "oafse/internal/domain/model"
	storage "oafse/internal/infrastructure/storage/model"
)

func ToDomainURL(u *storage.URLCache) (*domain.URL, error) {
	parsed, err := url.Parse(u.URL)
	if err != nil {
		return nil, err
	}
	return &domain.URL{
		URL:     parsed,
		Try:     u.Try,
		RetryAt: u.RetryAt,
	}, nil
}

func ToStorageURL(u *domain.URL) *storage.URLCache {
	return &storage.URLCache{
		URL:     u.String(),
		Try:     u.Try,
		RetryAt: u.RetryAt,
	}
}

func ToDomainCrawlMetadata(md *storage.CrawlMetadata) *port.CrawlMetadata {
	return &port.CrawlMetadata{
		UnprocessedCount: md.UnprocessedCount,
		EarliestRetry:    md.EarliestRetry,
	}
}
