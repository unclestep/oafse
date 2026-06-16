package repo

import (
	"context"
	"fmt"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

type CrawlRepo struct {
	urlRepoCache *URLRepoCache
	pageRepoDB   *PageRepoDB
}

func NewCrawlRepo(URLRepoCache *URLRepoCache, pageRepoDB *PageRepoDB) *CrawlRepo {
	return &CrawlRepo{
		urlRepoCache: URLRepoCache,
		pageRepoDB:   pageRepoDB,
	}
}

func (r *CrawlRepo) Start(ctx context.Context, url string) error {
	return r.urlRepoCache.Start(ctx, url)
}

func (r *CrawlRepo) StartHealthChecking(ctx context.Context, cfg *model.CrawlConfig) {
	r.urlRepoCache.StartHealthChecking(ctx, cfg)
}

func (r *CrawlRepo) TakeOn(ctx context.Context, workerID string) (*model.URL, error) {
	return r.urlRepoCache.TakeOn(ctx, workerID)
}

func (r *CrawlRepo) PushURLs(ctx context.Context, urls []string) ([]string, error) {
	return r.urlRepoCache.PushURLs(ctx, urls)
}

func (r *CrawlRepo) RetryURL(ctx context.Context, url string, retryAt time.Time) error {
	return r.urlRepoCache.RetryURL(ctx, url, retryAt)
}

func (r *CrawlRepo) GiveUpURL(ctx context.Context, url string) error {
	return r.urlRepoCache.GiveUpURL(ctx, url)
}

func (r *CrawlRepo) Done(ctx context.Context, page *model.Page) error {
	wrap := func(err error) error {
		return fmt.Errorf("crawl repo: done: %w", err)
	}
	if err := r.pageRepoDB.SavePage(ctx, page); err != nil {
		return wrap(err)
	}
	if _, err := r.urlRepoCache.PushURLs(ctx, page.Links); err != nil {
		return wrap(err)
	}
	if err := r.urlRepoCache.Done(ctx, page.URL); err != nil {
		return wrap(err)
	}
	return nil
}

func (r *CrawlRepo) GetCrawlMetadata(ctx context.Context) (*port.CrawlMetadata, error) {
	return r.urlRepoCache.GetCrawlMetadata(ctx)
}

func (r *CrawlRepo) ResetCrawlCache(ctx context.Context) error {
	return r.urlRepoCache.ResetCrawlCache(ctx)
}
