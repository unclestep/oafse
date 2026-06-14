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
	TakeOn(ctx context.Context, workerID int) (*model.URL, error)
	PushURLs(ctx context.Context, urls []*model.URL) ([]*model.URL, error)
	Retry(ctx context.Context, url *model.URL) error
	Done(ctx context.Context, page *model.Page) error
	GetCrawlMetadata(ctx context.Context) *CrawlMetadata
}
