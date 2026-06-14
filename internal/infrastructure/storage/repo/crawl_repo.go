package repo

import (
	"context"
	"fmt"

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

func (r *CrawlRepo) TakeOn(ctx context.Context, workerID int) (*model.URL, error) {
	return r.urlRepoCache.TakeOn(ctx, workerID)
}

func (r *CrawlRepo) PushURLs(ctx context.Context, urls []*model.URL) ([]*model.URL, error) {
	return r.urlRepoCache.PushURLs(ctx, urls)
}

func (r *CrawlRepo) Retry(ctx context.Context, url *model.URL) error {
	return r.urlRepoCache.Retry(ctx, url)
}

func (r *CrawlRepo) Done(ctx context.Context, page *model.Page) error {
	wrap := func(err error) error {
		return fmt.Errorf("crawl repo: done: %w", err)
	}
	err := r.urlRepoCache.Done(ctx, page.URL)
	if err != nil {
		return wrap(err)
	}
	err = r.pageRepoDB.SavePage(ctx, page)
	if err != nil {
		return wrap(err)
	}
	return nil
}
