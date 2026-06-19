package redis

import (
	"context"
	"fmt"
	"time"

	"oafse/internal/application/port"
	storage "oafse/internal/infrastructure/storage/model"

	"github.com/redis/go-redis/v9"
)

type Queue struct {
	rdb *redis.Client
}

func NewQueue(rdb *redis.Client) *Queue {
	return &Queue{rdb: rdb}
}

func (q *Queue) queueInsertChannel() string {
	return fmt.Sprintf("__keyspace@%d__:%s", q.rdb.Options().DB, KeyQueue)
}

func (q *Queue) Subscribe(ctx context.Context) (<-chan struct{}, error) {
	pubsub := q.rdb.Subscribe(ctx, q.queueInsertChannel())

	if _, err := pubsub.Receive(ctx); err != nil {
		pubsub.Close() //nolint:errcheck
		return nil, fmt.Errorf("subscribe: %w", err)
	}

	notify := make(chan struct{}, 1)

	go func() {
		defer pubsub.Close() //nolint:errcheck

		ch := pubsub.Channel()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				if msg.Payload != "lpush" {
					continue
				}
				select {
				case notify <- struct{}{}:
				default:
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	return notify, nil
}

var popURLScript = redis.NewScript(`
	local keyQueue = KEYS[1]
	local keyURLStatus = KEYS[2]
	local keyProcessingQueue = KEYS[3]
	local keyProcessingIndex = KEYS[4]
	local workerID = ARGV[1]
	local takeOnAt = ARGV[2]
	local newStatus = ARGV[3]

	local url = redis.call("LMOVE", keyQueue, keyProcessingQueue, "RIGHT", "LEFT")
	if not url then
		return nil
	end

	local oldJSON = redis.call('HGET', keyURLStatus, url)
	local info = cjson.decode(oldJSON)

	info['status'] = tonumber(newStatus)
	info['worker_id'] = workerID
	info['take_on_at'] = tonumber(takeOnAt)

	local newJSON = cjson.encode(info)
	redis.call('HSET', keyURLStatus, url, newJSON)

	redis.call('SADD', keyProcessingIndex, url)

	return {url, info['try'], info['parent']}
`)

func (q *Queue) TakeOn(ctx context.Context, workerID string) (*storage.URLCache, error) {
	keyProcessingQueue := WorkerKey(workerID)

	vals, err := popURLScript.Run(
		ctx, q.rdb,
		[]string{KeyQueue, KeyURLStatus, keyProcessingQueue, KeyProcessingIndex},
		workerID, time.Now().UnixMilli(), int(storage.ProcessingCrawlStatus),
	).Slice()
	if err == redis.Nil {
		return nil, port.ErrQueueEmpty
	}
	if err != nil {
		return nil, fmt.Errorf("take on: %w", err)
	}

	url, ok := vals[0].(string)
	if !ok {
		return nil, fmt.Errorf("take on: unexpected url type %T", vals[0])
	}

	try, ok := vals[1].(int64)
	if !ok {
		return nil, fmt.Errorf("take on: unexpected try type %T", vals[1])
	}

	parent, ok := vals[2].(string)
	if !ok {
		return nil, fmt.Errorf("take on: unexpected parent type %T", vals[2])
	}

	return &storage.URLCache{
		URL:    url,
		Try:    int(try),
		Parent: parent,
	}, nil
}

var pushURLScript = redis.NewScript(`
	local keyURLStatus = KEYS[1]
	local keyQueue = KEYS[2]
	local url = ARGV[1]
	local parent = ARGV[2]
	local status = ARGV[3]

	local isSeen = redis.call('HGET', keyURLStatus, url)

	if isSeen then
		return 0
	end

	local info = {
		status = tonumber(status),
		worker_id = '',
		try = 0,
		take_on_at = -1,
		retry_at = -1,
		parent = parent,
	}

	local pushedJSON = cjson.encode(info)

	redis.call('LPUSH', keyQueue, url)
	redis.call('HSET', keyURLStatus, url, pushedJSON)
	return 1
`)

func (q *Queue) PushURL(ctx context.Context, url *storage.URLCache) (bool, error) {
	res, err := pushURLScript.Run(
		ctx, q.rdb,
		[]string{KeyURLStatus, KeyQueue},
		url.URL, url.Parent, int(storage.QueuedCrawlStatus),
	).Int()
	return res == 1, err
}

func (q *Queue) PushURLs(ctx context.Context, urls []*storage.URLCache) ([]*storage.URLCache, error) {
	pipe := q.rdb.Pipeline()

	cmds := make([]*redis.Cmd, len(urls))
	for i, url := range urls {
		cmds[i] = pushURLScript.Eval(
			ctx, pipe,
			[]string{KeyURLStatus, KeyQueue},
			url.URL, url.Parent, int(storage.QueuedCrawlStatus),
		)
	}

	_, err := pipe.Exec(ctx)
	if err != nil {
		return nil, err
	}

	var pushed []*storage.URLCache
	for i, cmd := range cmds {
		res, err := cmd.Int()
		if err != nil || res != 1 {
			continue
		}
		pushed = append(pushed, urls[i])
	}

	return pushed, nil
}
