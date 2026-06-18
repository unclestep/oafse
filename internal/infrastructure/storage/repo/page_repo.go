package repo

import (
	"context"
	"fmt"

	domain "oafse/internal/domain/model"
	"oafse/internal/infrastructure/storage/ds"
	"oafse/internal/infrastructure/storage/mapper"
)

type PageRepoDB struct {
	s      ds.PageDBDS
	notify ds.PageNotifyDS
}

func NewPageRepoDB(pageDBDS ds.PageDBDS, pageNotifyDS ds.PageNotifyDS) *PageRepoDB {
	return &PageRepoDB{
		s:      pageDBDS,
		notify: pageNotifyDS,
	}
}

func (r *PageRepoDB) SavePage(ctx context.Context, page *domain.Page) (int64, error) {
	wrap := func(err error) error {
		return fmt.Errorf("page repo db: insert page: %w", err)
	}
	storagePage := mapper.ToStoragePage(page)
	pageID, err := r.s.InsertPage(ctx, storagePage)
	if err != nil {
		return -1, wrap(err)
	}
	return pageID, nil
}

func (r *PageRepoDB) SaveLink(ctx context.Context, parentURL string, childPageID int64) error {
	wrap := func(err error) error {
		return fmt.Errorf("page repo db: insert link: %w", err)
	}
	err := r.s.InsertLink(ctx, parentURL, childPageID)
	if err != nil {
		return wrap(err)
	}
	return nil
}

func (r *PageRepoDB) GetUnvectorized(ctx context.Context) ([]*domain.Page, error) {
	storagePages, err := r.s.GetUnvectorized(ctx)
	if err != nil {
		return nil, fmt.Errorf("get unvectorized: %w", err)
	}

	domainPages := make([]*domain.Page, len(storagePages))
	for i, page := range storagePages {
		domainPages[i] = mapper.ToDomainPage(page)
	}

	return domainPages, nil
}

func (r *PageRepoDB) StartListeningPages(ctx context.Context) (chan bool, chan error) {
	return r.notify.StartListening(ctx, "page_inserted")
}

func (r *PageRepoDB) FindSimilar(ctx context.Context, queryVector []float32, limit int) ([]*domain.Page, error) {
	storagePages, err := r.s.FindSimilar(ctx, queryVector, limit)
	if err != nil {
		return nil, fmt.Errorf("find similar: %w", err)
	}

	domainPages := make([]*domain.Page, len(storagePages))
	for i, page := range storagePages {
		domainPages[i] = mapper.ToDomainPage(page)
	}

	return domainPages, nil
}
