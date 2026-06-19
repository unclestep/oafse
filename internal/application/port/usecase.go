package port

import (
	"context"
	"time"

	"oafse/internal/domain/model"
)

type ParseUseCase interface {
	Execute(ctx context.Context, workerID string) (*ParseCmd, error)
}

type IndexUseCase interface {
	Execute(ctx context.Context)
}

type QueryUseCase interface {
	Execute(ctx context.Context, query string, limit int) ([]*model.Page, error)
}

type ParseCmd struct {
	SleepFor time.Duration
}
