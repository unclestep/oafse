package port

import (
	"time"

	"oafse/internal/domain/model"
)

type Processing interface {
	DecideRetry(url *model.URL, conf *CrawlConfig) (bool, time.Time)
}
