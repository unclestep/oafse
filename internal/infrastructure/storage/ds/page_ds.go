package ds

import (
	"context"

	storage "oafse/internal/infrastructure/storage/model"
)

type PageDBDS interface {
	SavePage(ctx context.Context, page *storage.PageDB) error
}
