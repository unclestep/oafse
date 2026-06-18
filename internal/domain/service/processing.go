package service

import (
	"math"
	"time"

	"oafse/internal/domain/model"
)

type Processing struct{}

func NewProcessing() *Processing {
	return &Processing{}
}

func (s *Processing) CalcRetryTime(try int, conf *model.CrawlConfig) (bool, time.Time) {
	if conf.TryLim != -1 && try >= conf.TryLim {
		return false, time.Time{}
	}

	retryAt := time.Now().Add(conf.TryBaseInterval * time.Duration(math.Pow(2, float64(try))))
	return true, retryAt
}
