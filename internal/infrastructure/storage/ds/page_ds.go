package ds

import (
	"context"

	storage "oafse/internal/infrastructure/storage/model"
)

type PageDBDS interface {
	InsertPage(ctx context.Context, page *storage.PageDB) (int64, error)
	InsertLink(ctx context.Context, parentURL string, childPageID int64) error
}
