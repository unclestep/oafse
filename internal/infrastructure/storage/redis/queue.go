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

func NewQueue(rdb *redis.Client) *Queue {
	return &Queue{rdb: rdb}
}

var popURLScript = redis.NewScript(`
	local keyQueue = KEYS[1]
	local keyURLStatus = KEYS[2]
	local keyProcessingQueue = KEYS[3]
	local keyProcessingIndex = KEYS[4]
	local workerID = ARGV[1]
	local processingStartTime = ARGV[2]
	local newStatus = ARGV[3]

	local url = redis.call("LMOVE", keyQueue, keyProcessingQueue, "RIGHT", "LEFT")
	if not url then
		return nil
	end

	local oldJSON = redis.call('HGET', keyURLStatus, url)
	local info = cjson.decode(oldJSON)

	info['status'] = newStatus
	info['worker_id'] = workerID
	info['processing_start_time'] = tonumber(processingStartTime)

	local newJSON = cjson.encode(info)
	redis.call('HSET', keyURLStatus, url, newJSON)

	redis.call('SADD', keyProcessingIndex, url)

	return url
`)

func (q *Queue) PopURL(ctx context.Context, workerID string) (string, error) {
	keyProcessingQueue := WorkerKey(workerID)

	url, err := popURLScript.Run(
		ctx, q.rdb,
		[]string{KeyQueue, KeyURLStatus, keyProcessingQueue, KeyProcessingIndex},
		workerID, time.Now().UnixMilli(), string(StatusProcessing),
	).Text()
	if err == redis.Nil {
		return "", nil
	}
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
		worker_id = '',
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
		[]string{KeyURLStatus, KeyQueue},
		url, string(StatusQueue),
	).Int()
	return res == 1, err
}

func (q *Queue) PushURLs(ctx context.Context, urls []string) ([]string, error) {
	pipe := q.rdb.Pipeline()

	cmds := make([]*redis.Cmd, len(urls))
	for i, url := range urls {
		cmds[i] = pushURLScript.Eval(
			ctx, pipe,
			[]string{KeyURLStatus, KeyQueue},
			url, string(StatusQueue),
		)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	var pushed []string
	for i, cmd := range cmds {
		res, err := cmd.Int()
		if err == nil && res == 1 {
			pushed = append(pushed, urls[i])
		}
	}

	return pushed, nil
}
