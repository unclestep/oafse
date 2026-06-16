package port

import (
	"context"
	"time"
)

type ParseUseCase interface {
	Execute(ctx context.Context, workerID string) (*ParseCmd, error)
}

type IndexUseCase interface {
	Execute(ctx context.Context)
}

type ParseCmd struct {
	Directive
	SleepFor time.Duration
}

type Directive int

const (
	DirectiveContinue Directive = iota
	DirectiveSleep
	DirectiveStop
)
