package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/infrastructure/storage/model"

	"github.com/redis/go-redis/v9"
)

type URLStatus string

const (
	StatusQueue      URLStatus = "queue"
	StatusProcessing URLStatus = "processing"
	StatusRetry      URLStatus = "retry"
	StatusProcessed  URLStatus = "processed"
	StatusFailure    URLStatus = "failure"
	StatusManual     URLStatus = "manual"
)

type URLInfo struct {
	Status   URLStatus `json:"status"`
	WorkerID string    `json:"worker_id"`
	TakeOnAt int64     `json:"take_on_at"` // UnixMilli format
	Try      int       `json:"try"`
	RetryAt  int64     `json:"retry_at"` // UnixMilli format
}

const (
	PrefixProcessingQueue = "crawler:processing:queue"
	KeyProcessingIndex    = "crawler:processing:index"
	KeyURLStatus          = "crawler:url:status"
	KeyQueue              = "crawler:queue"
	KeyRetry              = "crawler:retry"
)

func WorkerKey(workerID string) string {
	return PrefixProcessingQueue + ":" + workerID
}

type URLDS struct {
	rdb *redis.Client
	*Queue
	*Processing
	*Retry
}

func NewURLDS(rdb *redis.Client, queue *Queue, processing *Processing, retry *Retry) *URLDS {
	return &URLDS{
		rdb:        rdb,
		Queue:      queue,
		Processing: processing,
		Retry:      retry,
	}
}

func (s *URLDS) StartHealthChecking(ctx context.Context, cfg *port.CrawlConfig) {
	go func(worryThreshold time.Duration) {
		for {
			_ = s.HealthCheck(ctx, worryThreshold)
		}
	}(cfg.HealthCheckWorryThreshold)
}

func (s *URLDS) GetURLInfo(ctx context.Context, url string) (*URLInfo, error) {
	var info URLInfo

	bytesInfo, err := s.rdb.HGet(ctx, KeyURLStatus, url).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("get url info: url not found")
	} else if err != nil {
		return nil, fmt.Errorf("get url info: %w", err)
	}

	if err := json.Unmarshal([]byte(bytesInfo), &info); err != nil {
		return nil, fmt.Errorf("get url info unmarshal: %w", err)
	}

	return &info, nil
}

// TakeOn - lazily enqueue urls from retry queue. Better way to sleep workers before earliest URL becomes available
func (s *URLDS) TakeOn(ctx context.Context, workerID int) (*model.URLCache, error) {
	_, err := s.EnqueueURLs(ctx)
	if err != nil {
		return nil, fmt.Errorf("take on: can't enqueue urls: %w", err)
	}
	return s.TakeOn(ctx, workerID)
}

func (s *URLDS) PushURLs(ctx context.Context, urls []string) ([]string, error) {
	return s.PushURLs(ctx, urls)
}

func (s *URLDS) RetryURL(ctx context.Context, url string, retryAt time.Time) error {
	return s.RetryURL(ctx, url, retryAt)
}

func (s *URLDS) Done(ctx context.Context, url string) error {
	return s.MarkProcessed(ctx, url, StatusProcessed)
}

var getCrawlMetadataScript = redis.NewScript(`
	local keyQueue = KEYS[1]
	local keyRetry = KEYS[2]
	local keyProcessingIndex = KEYS[3]

	local queueCount = redis.call('LLEN', keyQueue)
	local retryCount = redis.call('ZCARD', keyRetry)
	local procCount = redis.call('SCARD', keyProcessingIndex)
	local total = queueCount + retryCount + procCount

	local range = redis.call('ZRANGE', keyRetry, 0, 0, 'WITHSCORES')

	if #range == 0 then
		return {total, false}
	end

	local earliest = range[2]

	return {total, earliest}
`)

func (s *URLDS) GetCrawlMetadata(ctx context.Context) (*model.CrawlMetadata, error) {
	vals, err := getCrawlMetadataScript.Run(
		ctx, s.rdb,
		[]string{KeyQueue, KeyRetry, KeyProcessingIndex},
	).Slice()
	if err == redis.Nil {
		return &model.CrawlMetadata{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get crawl metadata: %w", err)
	}

	unprocessed, ok := vals[0].(int64)
	if !ok {
		return nil, fmt.Errorf("get crawl metadata: unexpected unprocessed urls type %T", vals[0])
	}

	var earliestRetry time.Time
	if vals[1] != nil {
		earliest, ok := vals[1].(int64)
		if !ok {
			return nil, fmt.Errorf("get crawl metadata: unexpected earliest type %T", vals[1])
		}
		earliestRetry = time.UnixMilli(earliest)
	}

	return &model.CrawlMetadata{
		UnprocessedCount: int(unprocessed),
		EarliestRetry:    earliestRetry,
	}, nil
}
