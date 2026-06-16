package ds

import (
	"context"

	storage "oafse/internal/infrastructure/storage/model"
)

type PageDBDS interface {
	InsertPage(ctx context.Context, page *storage.PageDB) (int64, error)
	InsertLink(ctx context.Context, parentURL string, childPageID int64) error
	GetUnvectorized(ctx context.Context) ([]*storage.PageDB, error)
}

type PageNotifyDS interface {
	WaitForNotification(ctx context.Context) error
}
