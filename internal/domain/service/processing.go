package service

import (
	"math"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

type Processing struct{}

func NewProcessing() *Processing {
	return &Processing{}
}

func (s *Processing) DecideRetry(url *model.URL, conf *port.ParseConfig) bool {
	if conf.TryLim != -1 && url.Try >= conf.TryLim-1 {
		url.Status = model.GiveUpCrawlStatus
		return false
	}

	url.Try++
	url.RetryAt = time.Now().Add(time.Duration(math.Pow(float64(conf.TryBaseInterval), float64(url.Try))))
	return true
}
