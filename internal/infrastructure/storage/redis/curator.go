package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

type Curator struct {
	rdb *redis.Client
	*Queue
	*Processing
	*Retry
}

func NewCurator(rdb *redis.Client, queue *Queue, processing *Processing, retry *Retry) *Curator {
	return &Curator{
		rdb:        rdb,
		Queue:      queue,
		Processing: processing,
		Retry:      retry,
	}
}

func (r *Curator) GetURLInfo(ctx context.Context, url string) (*URLInfo, error) {
	var info URLInfo

	bytesInfo, err := r.rdb.HGet(ctx, KeyURLStatus, url).Result()
	if err == redis.Nil {
		return nil, fmt.Errorf("get url info: url not found")
	} else if err != nil {
		return nil, fmt.Errorf("get url info: %w", err)
	}

	if err := json.Unmarshal([]byte(bytesInfo), &info); err != nil {
		return nil, fmt.Errorf("get url info unmarshal: %w", err)
	}

	return &info, nil
}
