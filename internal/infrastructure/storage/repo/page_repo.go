package repo

import (
	"context"
	"fmt"

	domain "oafse/internal/domain/model"
	"oafse/internal/infrastructure/storage/ds"
	"oafse/internal/infrastructure/storage/mapper"
)

type PageRepoDB struct {
	s ds.PageDBDS
}

func NewPageRepoDB(source ds.PageDBDS) *PageRepoDB {
	return &PageRepoDB{
		s: source,
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
