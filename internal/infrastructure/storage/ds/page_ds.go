package ds

import (
	"context"

	storage "oafse/internal/infrastructure/storage/model"
)

type PageDBDS interface {
	InsertPage(ctx context.Context, page *storage.PageDB) (int64, error)
	InsertLink(ctx context.Context, parentURL string, childPageID int64) error
	GetUnvectorized(ctx context.Context) ([]*storage.PageDB, error)
	FindSimilar(ctx context.Context, queryVector []float32, limit int) ([]*storage.PageDB, error)
}

type PageNotifyDS interface {
	StartListening(ctx context.Context, music string) (chan bool, chan error)
}
