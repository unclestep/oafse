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

func (s *Processing) DecideRetry(url *model.URL, conf *model.CrawlConfig) (bool, time.Time) {
	url.Try++

	if conf.TryLim != -1 && url.Try >= conf.TryLim {
		return false, time.Time{}
	}

	retryAt := time.Now().Add(conf.TryBaseInterval * time.Duration(math.Pow(2, float64(url.Try))))
	return true, retryAt
}
