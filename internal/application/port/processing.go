package port

import (
	"time"

	"oafse/internal/domain/model"
)

type ParseConfig struct {
	TryLim          int
	TryBaseInterval time.Duration
}

type Processing interface {
	DecideRetry(url *model.URL, conf *ParseConfig) bool
}
