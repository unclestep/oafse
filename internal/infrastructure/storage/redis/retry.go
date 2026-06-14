package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Retry struct {
	rdb *redis.Client
}

func NewRetry(rdb *redis.Client) *Retry {
	return &Retry{rdb: rdb}
}

var enqueueURLsScript = redis.NewScript(`
	local keyRetry = KEYS[1]
	local keyURLStatus = KEYS[2]
	local keyQueue = KEYS[3]
	local now = ARGV[1]
	local newStatus = ARGV[2]

	local urls = redis.call('ZRANGE', keyRetry, '-inf', now, 'BYSCORE')
	if #urls == 0 then
		return redis.call('ZCARD', keyRetry)
	end

	redis.call('ZREMRANGEBYSCORE', keyRetry, '-inf', now)

	for i = 1, #urls do
		local binInfo = redis.call('HGET', keyURLStatus, urls[i])
		local info
		if not binInfo then
			info = {
				status = newStatus,
				worker_id = "",
				try = 1,
				take_on_at = -1,
				retry_at = -1,
			}
		else
			info = cjson.decode(binInfo)
		end

		info['status'] = newStatus
		local newBinInfo = cjson.encode(info)
		redis.call('HSET', keyURLStatus, urls[i], newBinInfo)
		redis.call('LPUSH', keyQueue, urls[i])
	end

	return redis.call('ZCARD', keyRetry)
`)

func (r *Retry) EnqueueURLs(ctx context.Context) (int64, error) {
	remaining, err := enqueueURLsScript.Run(
		ctx, r.rdb,
		[]string{KeyRetry, KeyURLStatus, KeyQueue},
		time.Now().UnixMilli(), string(StatusQueue),
	).Int64()
	if err != nil && err != redis.Nil {
		return 0, fmt.Errorf("enqueue retry urls: %w", err)
	}
	return remaining, nil
}
