package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Processing struct {
	rdb     *redis.Client
	workers int
}

var retryURLScript = redis.NewScript(`
	local keyURLStatus = KEYS[1]
	local keyProcessingIndex = KEYS[2]
	local keyRetry = KEYS[3]
	local url = ARGV[1]
	local prefixProcessingQueue = ARGV[2]
	local newStatus = ARGV[3]
	local next_retry_time = ARGV[4]

	local oldJSON = redis.call('HGET', keyURLStatus, url)
	local info = cjson.decode(oldJSON)

	local keyProcessingQueue = prefixProcessingQueue .. ':' .. info['worker_id']
	info['status'] = newStatus
	info['tries'] = info['tries'] + 1
	info['next_retry_time'] = next_retry_time

	local newJSON = cjson.encode(info)

	redis.call('HSET', keyURLStatus, url, newJSON)
	redis.call('LREM', keyProcessingQueue, 1, url)
	redis.call('SREM', keyProcessingIndex, url)
	redis.call('ZADD', keyRetry, next_retry_time, url)
`)

func (p *Processing) RetryURL(ctx context.Context, url string, nextRetryTime time.Time) error {
	err := retryURLScript.Run(
		ctx, p.rdb,
		[]string{keyURLStatus, keyProcessingIndex, keyRetry},
		url, prefixProcessingQueue, StatusRetry, nextRetryTime.UnixNano(),
	).Err()
	if err != nil {
		return fmt.Errorf("retry url: %w", err)
	}
	return nil
}

var markProcessedScript = redis.NewScript(`
	local keyURLStatus = KEYS[1]
	local keyProcessingIndex = KEYS[2]
	local url = ARGV[1]
	local prefixProcessingQueue = ARGV[2]
	local newStatus = ARGV[3]

	local oldJSON = redis.call('HGET', keyURLStatus, url)
	local info = cjson.decode(oldJSON)

	local keyProcessingQueue = prefixProcessingQueue .. ':' .. info['worker_id']
	info['status'] = newStatus

	local newJSON = cjson.encode(info)

	redis.call('HSET', keyURLStatus, url, newJSON)
	redis.call('LREM', keyProcessingQueue, 1, url)
	redis.call('SREM', keyProcessingIndex, url)
`)

func (p *Processing) MarkProcessed(ctx context.Context, url string) error {
	err := markProcessedScript.Run(
		ctx, p.rdb,
		[]string{keyURLStatus, keyProcessingIndex},
		url, prefixProcessingQueue, StatusProcessed,
	).Err()
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

var recoverScript = redis.NewScript(`
	local keyURLStatus = KEYS[1]
	local keyProcessingIndex = KEYS[2]
	local keyQueue = KEYS[3]
	local url = ARGV[1]
	local prefixProcessingQueue = ARGV[2]
	local newStatus = ARGV[3]

	local oldJSON = redis.call('HGET', keyURLStatus, url)

	if not oldJSON then
		local info = {
			status = newStatus,
			worker_id = -1,
			tries = 0,
			processing_start_time = -1,
			next_retry_time = -1,
		}
		local binInfo = cjson.encode(info)

		redis.call('SREM', keyProcessingIndex, url)
		redis.call('LPUSH', keyQueue, url)
		redis.call('HSET', keyURLStatus, url, binInfo)

		return 1
	end

	local info = cjson.decode(oldJSON)

	local keyProcessingQueue = prefixProcessingQueue .. ':' .. info['worker_id']
	info['status'] = newStatus

	local newJSON = cjson.encode(info)

	redis.call('HSET', keyURLStatus, url, newJSON)
	redis.call('LREM', keyProcessingQueue, 1, url)
	redis.call('SREM', keyProcessingIndex, url)
	redis.call('LPUSH', keyQueue, url)

	return 1
`)

func (p *Processing) RecoverURL(ctx context.Context, url string) error {
	err := recoverScript.Run(
		ctx, p.rdb,
		[]string{keyURLStatus, keyProcessingIndex, keyQueue},
		url, prefixProcessingQueue, StatusQueue,
	).Err()
	if err != nil {
		return fmt.Errorf("recover url: %w", err)
	}
	return nil
}

func (p *Processing) HealthCheck(ctx context.Context, worryThreshold time.Duration) error {
	var cursor uint64
	for {
		urls, nextCursor, err := p.rdb.SScan(ctx, keyProcessingIndex, cursor, "", int64(p.workers)).Result()

		if err != nil {
			return fmt.Errorf("health check sscan: %w", err)
		}

		for _, url := range urls {
			bytesJSON, err := p.rdb.HGet(ctx, keyURLStatus, url).Result()
			if err == redis.Nil {
				if err := p.RecoverURL(ctx, url); err != nil {
					return fmt.Errorf("health check recover orphaned url: %w", err)
				}
				continue
			} else if err != nil {
				return fmt.Errorf("health check hget: %w", err)
			}

			var info URLInfo
			if err := json.Unmarshal([]byte(bytesJSON), &info); err != nil {
				return fmt.Errorf("health check unmarshal: %w", err)
			}

			if time.Now().UnixNano()-info.ProcessingStartTime >= worryThreshold.Nanoseconds() {
				if err := p.RecoverURL(ctx, url); err != nil {
					return fmt.Errorf("health check recover url: %w", err)
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}
