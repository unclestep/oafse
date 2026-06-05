package redis_test

import (
	"context"
	redis "github.com/redis/go-redis/v9"
	"time"

	sredis "oafse/internal/redis"
)

func (s *RedisSuite) TestEnqueueURLsBasic() {
	url := [][]string{
		{"URL1", "URL2"},
	}

	s.prepareProcessingQueue(url)
	err := s.curator.RetryURL(context.Background(), "URL1", time.Now().Add(time.Millisecond))
	s.NoError(err)
	err = s.curator.RetryURL(context.Background(), "URL2", time.Now().Add(10*time.Millisecond))
	s.NoError(err)
	time.Sleep(5 * time.Millisecond)

	remain, err := s.curator.EnqueueURLs(context.Background())
	s.NoError(err)
	s.Equal(int64(1), remain)

	llen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
	s.NoError(err)
	s.Equal(int64(1), llen)

	info, err := s.curator.GetURLInfo(context.Background(), "URL1")
	if !s.NoError(err) {
		return
	}
	s.Equal(sredis.StatusQueue, info.Status)
	s.Equal(1, info.Tries)

	info2, err := s.curator.GetURLInfo(context.Background(), "URL2")
	s.NoError(err)
	s.Equal(sredis.StatusRetry, info2.Status)

	// Idempotent
	remain, err = s.curator.EnqueueURLs(context.Background())
	s.NoError(err)
	s.Equal(int64(1), remain)
	llen, err = s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
	s.NoError(err)
	s.Equal(int64(1), llen)

	// Enqueue all
	time.Sleep(5 * time.Millisecond)
	remain, err = s.curator.EnqueueURLs(context.Background())
	s.NoError(err)
	s.Equal(int64(0), remain)

	llen, err = s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
	s.NoError(err)
	s.Equal(int64(2), llen)
}

func (s *RedisSuite) TestEnqueueURLsEmptyRetry() {
	remain, err := s.curator.EnqueueURLs(context.Background())
	s.NoError(err)
	s.Equal(int64(0), remain)

	llen, _ := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
	s.Equal(int64(0), llen)
}

func (s *RedisSuite) TestEnqueueURLsNoStatusEntry() {
	s.rdb.ZAdd(context.Background(), sredis.KeyRetry, redis.Z{
		Score:  float64(time.Now().Add(-time.Second).UnixMilli()),
		Member: "ORPHAN",
	})

	remain, err := s.curator.EnqueueURLs(context.Background())
	s.NoError(err)
	s.Equal(int64(0), remain)

	info, err := s.curator.GetURLInfo(context.Background(), "ORPHAN")
	s.NoError(err)
	s.Equal(sredis.StatusQueue, info.Status)
}
