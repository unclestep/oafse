package port

import (
	"context"
	"errors"

	"oafse/internal/domain/model"
)

var ErrQueueEmpty = errors.New("queue is empty")

type PageRepo interface {
	TakeOn(ctx context.Context) (*model.URL, error)
	Retry(ctx context.Context, url *model.URL) error
	Done(ctx context.Context, page *model.Page) error
}
