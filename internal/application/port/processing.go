package port

import (
	"time"

	"oafse/internal/domain/model"
)

type Processing interface {
	CalcRetryTime(try int, conf *model.CrawlConfig) (bool, time.Time)
}
