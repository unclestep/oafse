package ds

import (
	"context"
	"time"

	domain "oafse/internal/domain/model"
	storage "oafse/internal/infrastructure/storage/model"
)

type URLCacheDS interface {
	Start(ctx context.Context, url string) error
	StartHealthChecking(ctx context.Context, cfg *domain.CrawlConfig)
	TakeOn(ctx context.Context, workerID string) (*storage.URLCache, error)
	PushURLs(ctx context.Context, urls []*storage.URLCache) ([]*storage.URLCache, error)
	RetryURL(ctx context.Context, url string, retryAt time.Time) error
	MarkProcessed(ctx context.Context, url string, status storage.CrawlStatus) error
	GetCrawlMetadata(ctx context.Context) (*storage.CrawlMetadata, error)
	ResetCrawlCache(ctx context.Context) error
}
