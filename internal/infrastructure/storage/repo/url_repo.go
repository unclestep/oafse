package repo

import (
	"context"
	"fmt"

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

func (r *URLRepoCache) TakeOn(ctx context.Context, workerID int) (*domain.URL, error) {
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
			continue
		}
		domainURLs[i] = domainURL
	}

	return domainURLs, nil
}

func (r *URLRepoCache) Retry(ctx context.Context, url *domain.URL) error {
	wrap := func(err error) error {
		return fmt.Errorf("retry url: %w", err)
	}

	storageURL := mapper.ToStorageURL(url)
	err := r.s.Retry(ctx, storageURL)
	if err != nil {
		return wrap(err)
	}
	return nil
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

func (r *URLRepoCache) GetCrawlMetadata(ctx context.Context) *port.CrawlMetadata {
	md := r.s.GetCrawlMeta(ctx)
	return mapper.ToDomainCrawlMetadata(md)
}
