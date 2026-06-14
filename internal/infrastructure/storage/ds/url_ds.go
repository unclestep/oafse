package ds

import (
	"context"
	"time"

	"oafse/internal/infrastructure/storage/model"
)

type URLCacheDS interface {
	TakeOn(ctx context.Context, workerID string) (*model.URLCache, error)
	PushURLs(ctx context.Context, urls []string) ([]string, error)
	RetryURL(ctx context.Context, url string, retryAt time.Time) error
	GiveUpURL(ctx context.Context, url string) error
	Done(ctx context.Context, url string) error
	GetCrawlMetadata(ctx context.Context) (*model.CrawlMetadata, error)
}
