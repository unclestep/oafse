package port

import (
	"context"
	"time"
)

type ParseUseCase interface {
	Execute(ctx context.Context) (*ParseCmd, error)
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
