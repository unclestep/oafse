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

type CrawlRepo interface {
	Start(ctx context.Context, url string) error
	StartHealthChecking(ctx context.Context, cfg *CrawlConfig)
	TakeOn(ctx context.Context, workerID string) (*model.URL, error)
	PushURLs(ctx context.Context, urls []string) ([]string, error)
	RetryURL(ctx context.Context, url string, retryAt time.Time) error
	GiveUpURL(ctx context.Context, url string) error
	Done(ctx context.Context, page *model.Page) error
	GetCrawlMetadata(ctx context.Context) (*CrawlMetadata, error)
}
