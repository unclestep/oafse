package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func NewClient(databaseURL string) (*redis.Client, error) {
	opt, err := redis.ParseURL(databaseURL)
	if err != nil {
		return nil, err
	}

	opt.PoolSize = 20
	opt.MinIdleConns = 4
	opt.MaxRetries = 5
	opt.DialTimeout = 15 * time.Second
	opt.DialerRetries = 5
	opt.DialerRetryTimeout = 100 * time.Millisecond
	opt.MinRetryBackoff = 100 * time.Millisecond
	opt.MaxRetryBackoff = 1 * time.Second

	rdb := redis.NewClient(opt)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		rdb.Close() //nolint:errcheck
		return nil, err
	}

	return rdb, nil
}
