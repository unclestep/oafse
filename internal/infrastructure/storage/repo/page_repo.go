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

func (r *PageRepoDB) SavePage(ctx context.Context, page *domain.Page) error {
	wrap := func(err error) error {
		return fmt.Errorf("page repo db: save page: %w", err)
	}
	storagePage := mapper.ToStoragePage(page)
	err := r.s.SavePage(ctx, storagePage)
	if err != nil {
		return wrap(err)
	}
	return nil
}
