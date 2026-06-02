package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type Queue struct {
	rdb *redis.Client
}

var popURLScript = redis.NewScript(`
	local keyURLStatus = KEYS[1]
	local keyProcessingQueue = KEYS[2]
	local keyProcessingIndex = KEYS[3]
	local url = ARGV[1]
	local workerID = ARGV[2]
	local processingStartTime = ARGV[3]

	local oldJSON = redis.call('HGET', keyURLStatus, url)
	local info = cjson.decode(oldJSON)

	info['status'] = keyProcessingQueue
	info['worker_id'] = workerID
	info['processing_start_time'] = processingStartTime

	local newJSON = cjson.encode(info)
	redis.call('HSET', url, newJSON)

	redis.call('SADD', keyProcessingIndex, url)
`)

func (q *Queue) PopURL(ctx context.Context, workerID string) (string, error) {
	keyProcessingQueue := workerKey(workerID)

	url, err := q.rdb.BLMove(ctx, keyQueue, keyProcessingQueue, "RIGHT", "LEFT", 15*time.Second).Result()
	if err != nil {
		return "", fmt.Errorf("pop url BLMOVE: %w", err)
	}

	err = popURLScript.Run(
		ctx, q.rdb,
		[]string{keyURLStatus, keyProcessingQueue, keyProcessingIndex},
		url, workerID, time.Now().UnixNano(),
	).Err()
	if err != nil {
		return "", fmt.Errorf("pop url lua: %w", err)
	}

	return url, nil
}

var pushURLScript = redis.NewScript(`
	local keyURLStatus = KEYS[1]
	local keyQueue = KEYS[2]
	local url = ARGV[1]
	local status = ARGV[2]

	local isSeen = redis.call('HGET', keyURLStatus, url)

	if isSeen then
		return 0
	end

	local info = {
		status = status,
		worker_id = -1,
		tries = 0,
		processing_start_time = -1,
		next_retry_time = -1,
	}

	local pushedJSON = cjson.encode(info)

	redis.call('LPUSH', keyQueue, url)
	redis.call('HSET', keyURLStatus, url, pushedJSON)
	return 1
`)

func (q *Queue) PushURL(ctx context.Context, url string) (bool, error) {
	res, err := pushURLScript.Run(
		ctx, q.rdb,
		[]string{keyURLStatus, keyQueue},
		url, StatusQueue,
	).Int()
	return res == 1, err
}
