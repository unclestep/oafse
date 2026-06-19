package repo

import (
	"context"
	"fmt"
	"time"

	"oafse/internal/application/port"
	domain "oafse/internal/domain/model"
	"oafse/internal/infrastructure/storage/ds"
	"oafse/internal/infrastructure/storage/mapper"
	storage "oafse/internal/infrastructure/storage/model"
)

type URLRepoCache struct {
	s ds.URLCacheDS
}

func NewURLRepoCache(source ds.URLCacheDS) *URLRepoCache {
	return &URLRepoCache{
		s: source,
	}
}

func (r *URLRepoCache) Start(ctx context.Context, url string) error {
	return r.s.Start(ctx, url)
}

func (r *URLRepoCache) StartHealthChecking(ctx context.Context, cfg *domain.CrawlConfig) {
	r.s.StartHealthChecking(ctx, cfg)
}

func (r *URLRepoCache) TakeOn(ctx context.Context, workerID string) (*domain.URL, error) {
	wrap := func(err error) error {
		return fmt.Errorf("take on: %w", err)
	}

	storageURL, err := r.s.TakeOn(ctx, workerID)
	if err != nil {
		return nil, wrap(err)
	}

	domainURL, err := mapper.ToDomainURL(storageURL)
	if err != nil {
		return nil, wrap(err)
	}
	return domainURL, nil
}

func (r *URLRepoCache) PushURLs(ctx context.Context, urls []*domain.URL) ([]*domain.URL, error) {
	wrap := func(err error) error {
		return fmt.Errorf("push urls: %w", err)
	}

	storageURLs := make([]*storage.URLCache, len(urls))
	for i, url := range urls {
		storageURLs[i] = mapper.ToStorageURL(url)
	}

	pushed, err := r.s.PushURLs(ctx, storageURLs)
	if err != nil {
		return nil, wrap(err)
	}

	domainURLs := make([]*domain.URL, len(pushed))
	for i, url := range pushed {
		domainURL, err := mapper.ToDomainURL(url)
		if err != nil {
			return nil, wrap(err)
		}
		domainURLs[i] = domainURL
	}

	return domainURLs, nil
}

func (r *URLRepoCache) RetryURL(ctx context.Context, url string, retryAt time.Time) error {
	wrap := func(err error) error {
		return fmt.Errorf("retry url: %w", err)
	}

	err := r.s.RetryURL(ctx, url, retryAt)
	if err != nil {
		return wrap(err)
	}
	return nil
}

func (r *URLRepoCache) MarkProcessed(ctx context.Context, url string, status domain.CrawlStatus) error {
	return r.s.MarkProcessed(ctx, url, mapper.ToStorageCrawlStatus(status))
}

func (r *URLRepoCache) GetCrawlMetadata(ctx context.Context) (*port.CrawlMetadata, error) {
	md, err := r.s.GetCrawlMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("get crawl metadata: %w", err)
	}
	return mapper.ToDomainCrawlMetadata(md), nil
}

func (r *URLRepoCache) ResetCrawlCache(ctx context.Context) error {
	return r.s.ResetCrawlCache(ctx)
}

func (r *URLRepoCache) Subscribe(ctx context.Context) (<-chan struct{}, error) {
	return r.s.Subscribe(ctx)
}
