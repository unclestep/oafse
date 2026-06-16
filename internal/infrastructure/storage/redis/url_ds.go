package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	domain "oafse/internal/domain/model"
	storage "oafse/internal/infrastructure/storage/model"

	"github.com/redis/go-redis/v9"
)

type URLInfo struct {
	Status   storage.CrawlStatus `json:"status"`
	WorkerID string              `json:"worker_id"`
	TakeOnAt int64               `json:"take_on_at"` // UnixMilli format
	Try      int                 `json:"try"`
	RetryAt  int64               `json:"retry_at"` // UnixMilli format
	Parent   string              `json:"parent"`
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

func (s *URLDS) Start(ctx context.Context, url string) error {
	metaURL := &storage.URLCache{URL: url}
	_, err := s.PushURL(ctx, metaURL)
	if err != nil {
		return fmt.Errorf("start: %w", err)
	}
	return nil
}

func (s *URLDS) StartHealthChecking(ctx context.Context, cfg *domain.CrawlConfig) {
	go func() {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("[WARN] health checking: %s", err)
				s.StartHealthChecking(ctx, cfg)
			}
		}()

		ticker := time.NewTicker(cfg.HealthCheckWorryThreshold)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				_ = s.HealthCheck(ctx, cfg.HealthCheckWorryThreshold)
			case <-ctx.Done():
				return
			}
		}
	}()
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
func (s *URLDS) TakeOn(ctx context.Context, workerID string) (*storage.URLCache, error) {
	_, err := s.EnqueueURLs(ctx)
	if err != nil {
		return nil, fmt.Errorf("take on: can't enqueue urls: %w", err)
	}
	return s.Queue.TakeOn(ctx, workerID)
}

func (s *URLDS) PushURLs(ctx context.Context, urls []*storage.URLCache) ([]*storage.URLCache, error) {
	return s.Queue.PushURLs(ctx, urls)
}

func (s *URLDS) RetryURL(ctx context.Context, url string, retryAt time.Time) error {
	return s.Processing.RetryURL(ctx, url, retryAt)
}

func (s *URLDS) MarkProcessed(ctx context.Context, url string, status storage.CrawlStatus) error {
	return s.Processing.MarkProcessed(ctx, url, status)
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

	local earliest = tonumber(range[2])

	return {total, earliest}
`)

func (s *URLDS) GetCrawlMetadata(ctx context.Context) (*storage.CrawlMetadata, error) {
	vals, err := getCrawlMetadataScript.Run(
		ctx, s.rdb,
		[]string{KeyQueue, KeyRetry, KeyProcessingIndex},
	).Slice()
	if err == redis.Nil {
		return &storage.CrawlMetadata{}, nil
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

	return &storage.CrawlMetadata{
		UnprocessedCount: int(unprocessed),
		EarliestRetry:    earliestRetry,
	}, nil
}

func (s *URLDS) ResetCrawlCache(ctx context.Context) error {
	var cursor uint64
	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, "crawler:*", 100).Result()
		if err != nil {
			return fmt.Errorf("reset crawl cache: %w", err)
		}
		if len(keys) > 0 {
			if err := s.rdb.Del(ctx, keys...).Err(); err != nil {
				return fmt.Errorf("reset crawl cache: %w", err)
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}
	return nil
}
