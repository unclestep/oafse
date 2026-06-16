package mapper

import (
	"net/url"

	"oafse/internal/application/port"
	domain "oafse/internal/domain/model"
	storage "oafse/internal/infrastructure/storage/model"
)

func ToDomainURL(u *storage.URLCache) (*domain.URL, error) {
	unnorm, err := url.Parse(u.URL)
	if err != nil {
		return nil, err
	}
	norm := domain.Normalize(unnorm)
	return &domain.URL{
		URL:    norm,
		Parent: u.Parent,
		Try:    u.Try,
	}, nil
}

func ToStorageURL(u *domain.URL) *storage.URLCache {
	return &storage.URLCache{
		URL:    u.String(),
		Parent: u.Parent,
		Try:    u.Try,
	}
}

func ToDomainCrawlMetadata(md *storage.CrawlMetadata) *port.CrawlMetadata {
	return &port.CrawlMetadata{
		UnprocessedCount: md.UnprocessedCount,
		EarliestRetry:    md.EarliestRetry,
	}
}

func ToStorageCrawlStatus(status domain.CrawlStatus) storage.CrawlStatus {
	switch status {
	case domain.QueuedCrawlStatus:
		return storage.QueuedCrawlStatus
	case domain.ProcessingCrawlStatus:
		return storage.ProcessingCrawlStatus
	case domain.DoneCrawlStatus:
		return storage.DoneCrawlStatus
	case domain.ManualCrawlStatus:
		return storage.ManualCrawlStatus
	case domain.GiveUpCrawlStatus:
		return storage.GiveUpCrawlStatus
	case domain.RetryCrawlStatus:
		return storage.RetryCrawlStatus
	default:
		return storage.UnknownCrawlStatus
	}
}
