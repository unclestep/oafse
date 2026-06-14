package ds

import (
	"context"

	"oafse/internal/infrastructure/storage/model"
)

type URLCacheDS interface {
	TakeOn(ctx context.Context, workerID int) (*model.URLCache, error)
	PushURLs(ctx context.Context, urls []*model.URLCache) ([]*model.URLCache, error)
	Retry(ctx context.Context, url *model.URLCache) error
	Done(ctx context.Context, url string) error
	GetCrawlMeta(ctx context.Context) *model.CrawlMetadata
}
