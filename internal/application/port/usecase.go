package port

import (
	"context"
	"time"
)

type CrawlConfig struct {
	StartURL                  string
	WorkersCount              int
	TryLim                    int
	TryBaseInterval           time.Duration
	HealthCheckWorryThreshold time.Duration
}

type ParseUseCase interface {
	Execute(ctx context.Context, workerID string) (*ParseCmd, error)
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
