package port

import (
	"context"
	"errors"
	"time"

	"oafse/internal/domain/model"
)

var (
	ErrQueueEmpty   = errors.New("queue is empty")
	ErrPageNotFound = errors.New("page not found")
)

type CrawlMetadata struct {
	UnprocessedCount int
	EarliestRetry    time.Time
}

type URLRepo interface {
	Start(ctx context.Context, url string) error
	StartHealthChecking(ctx context.Context, cfg *model.CrawlConfig)
	TakeOn(ctx context.Context, workerID string) (*model.URL, error)
	PushURLs(ctx context.Context, urls []*model.URL) ([]*model.URL, error)
	RetryURL(ctx context.Context, url string, retryAt time.Time) error
	MarkProcessed(ctx context.Context, url string, status model.CrawlStatus) error
	GetCrawlMetadata(ctx context.Context) (*CrawlMetadata, error)
	ResetCrawlCache(ctx context.Context) error
}

type PageDBRepo interface {
	SavePage(ctx context.Context, page *model.Page) (int64, error)
	SaveLink(ctx context.Context, parentURL string, childPageID int64) error
}
