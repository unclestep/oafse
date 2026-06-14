package repo

import (
	"context"
	"fmt"
	"time"

	"oafse/internal/application/port"
	domain "oafse/internal/domain/model"
	"oafse/internal/infrastructure/storage/ds"
	"oafse/internal/infrastructure/storage/mapper"
)

type URLRepoCache struct {
	s ds.URLCacheDS
}

func NewURLRepoCache(source ds.URLCacheDS) *URLRepoCache {
	return &URLRepoCache{
		s: source,
	}
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

func (r *URLRepoCache) PushURLs(ctx context.Context, urls []string) ([]string, error) {
	wrap := func(err error) error {
		return fmt.Errorf("push urls: %w", err)
	}

	pushed, err := r.s.PushURLs(ctx, urls)
	if err != nil {
		return nil, wrap(err)
	}

	return pushed, nil
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

func (r *URLRepoCache) GiveUpURL(ctx context.Context, url string) error {
	return r.s.GiveUpURL(ctx, url)
}

func (r *URLRepoCache) Done(ctx context.Context, url string) error {
	wrap := func(err error) error {
		return fmt.Errorf("done: %w", err)
	}

	err := r.s.Done(ctx, url)
	if err != nil {
		return wrap(err)
	}
	return nil
}

func (r *URLRepoCache) GetCrawlMetadata(ctx context.Context) (*port.CrawlMetadata, error) {
	md, err := r.s.GetCrawlMetadata(ctx)
	if err != nil {
		return nil, fmt.Errorf("get crawl metadata: %w", err)
	}
	return mapper.ToDomainCrawlMetadata(md), nil
}
