package redis_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"oafse/internal/application/port"
	sredis "oafse/internal/infrastructure/storage/redis"
)

func (s *RedisSuite) TestGetURL() {
	s.Run("Get nonexistent URL", func() {
		urlRet, err := s.curator.GetURL(context.Background(), "0")
		s.NoError(err)
		s.Equal("", urlRet)
	})

	s.Run("Get valid URL", func() {
		url := [][]string{
			{"URL"},
		}
		s.prepareProcessingQueue(url)

		urlRet, err := s.curator.GetURL(context.Background(), "0")
		s.NoError(err)
		s.Equal("URL", urlRet)
	})
}

func (s *RedisSuite) TestRetryURL() {
	s.Run("Nonexistent URL Retry", func() {
		err := s.curator.RetryURL(context.Background(), "INVALID", time.Now())
		s.Error(err)

		zcard, err := s.rdb.ZCard(context.Background(), sredis.KeyRetry).Result()
		s.NoError(err)
		s.Equal(int64(0), zcard)
	})

	s.Run("Real URL Retry", func() {
		url := [][]string{
			{"URL"},
		}

		s.prepareProcessingQueue(url)
		retryTime := time.Now().Add(100 * time.Millisecond)
		err := s.curator.RetryURL(context.Background(), "URL", retryTime)
		s.NoError(err)

		info, err := s.curator.GetURLInfo(context.Background(), "URL")
		if !s.NoError(err) {
			return
		}

		s.Equal(sredis.StatusRetry, info.Status)
		s.Equal(1, info.Try)
		s.Equal(retryTime.UnixMilli(), info.RetryAt)

		score, err := s.rdb.ZScore(context.Background(), sredis.KeyRetry, "URL").Result()
		s.NoError(err)
		s.Equal(float64(retryTime.UnixMilli()), score)
	})
}

func (s *RedisSuite) TestRetryURLConcurrent() {
	urls := s.pushURLs()
	wCount := len(urls)

	var wg sync.WaitGroup

	wg.Add(wCount)
	for range wCount {
		go func() {
			defer wg.Done()
			workerID := fmt.Sprint(rand.Int31n(int32(WorkersCount)))
			for {
				cache, err := s.curator.TakeOn(context.Background(), workerID)
				if errors.Is(err, port.ErrQueueEmpty) {
					break
				}
				if !s.NoError(err) {
					return
				}

				info, err := s.curator.GetURLInfo(context.Background(), cache.URL)
				if !s.NoError(err) {
					return
				}

				s.Equal(sredis.StatusProcessing, info.Status, "URL Status")
				s.Equal(workerID, info.WorkerID, "Worker ID")
				s.Greater(info.TakeOnAt, time.Now().UnixMilli()-time.Minute.Milliseconds(), "Processing Start Time")
			}
		}()
	}
	wg.Wait()

	wg.Add(wCount)
	for i := range wCount {
		go func() {
			defer wg.Done()
			for j := range len(urls[i]) {
				retryTime := time.Now().Add(100 * time.Millisecond)

				err := s.curator.RetryURL(context.Background(), urls[i][j], retryTime)
				if !s.NoError(err) {
					return
				}

				info, err := s.curator.GetURLInfo(context.Background(), urls[i][j])
				if !s.NoError(err) {
					return
				}

				s.Equal(sredis.StatusRetry, info.Status)
				s.Equal(1, info.Try)
				s.Equal(retryTime.UnixMilli(), info.RetryAt)
			}
		}()
	}
	wg.Wait()

	for i := range wCount {
		llen, err := s.rdb.LLen(context.Background(), sredis.WorkerKey(fmt.Sprint(i))).Result()
		s.NoError(err)
		s.Equal(int64(0), llen)
	}

	sLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
	s.NoError(err)
	s.Equal(int64(0), sLen)

	zLen, err := s.rdb.ZCard(context.Background(), sredis.KeyRetry).Result()
	s.NoError(err)
	s.Equal(int64(len(urls)*len(urls[0])), zLen)
}

func (s *RedisSuite) TestMarkProcessed() {
	urls := [][]string{
		{"1", "2", "3"},
	}

	tests := [][]string{
		{string(sredis.StatusProcessed), "1"},
		{string(sredis.StatusFailure), "2"},
		{string(sredis.StatusManual), "3"},
	}

	s.prepareProcessingQueue(urls)

	for i := range len(tests) {
		s.Run(fmt.Sprintf("%s -> %s", tests[i][1], tests[i][0]), func() {
			err := s.curator.MarkProcessed(context.Background(), tests[i][1], sredis.URLStatus(tests[i][0]))
			s.NoError(err)

			info, err := s.curator.GetURLInfo(context.Background(), tests[i][1])
			s.NoError(err)

			s.Equal(sredis.URLStatus(tests[i][0]), info.Status)
		})
	}

	llen, err := s.rdb.LLen(context.Background(), sredis.WorkerKey("0")).Result()
	s.NoError(err)
	s.Equal(int64(0), llen)

	scard, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
	s.NoError(err)
	s.Equal(int64(0), scard)

	s.Run("Nonexistent URL", func() {
		err := s.curator.MarkProcessed(context.Background(), "NOURL", sredis.StatusProcessed)
		s.Error(err)
	})
}

func (s *RedisSuite) TestRecoverSuite() {
	s.Run("URL Not Found", func() {
		defer s.rdb.FlushDB(context.Background())

		err := s.curator.RecoverURL(context.Background(), "NEWURL")
		s.NoError(err)

		info, err := s.curator.GetURLInfo(context.Background(), "NEWURL")
		s.NoError(err)
		s.Equal(sredis.StatusQueue, info.Status)
		s.Equal("", info.WorkerID)
		s.Equal(0, info.Try)

		queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
		s.NoError(err)
		s.Equal(int64(1), queueLen)
	})

	s.Run("URL exists in preprocessing", func() {
		defer s.rdb.FlushDB(context.Background())

		urls := [][]string{{"URL"}}
		s.prepareProcessingQueue(urls)

		err := s.curator.RecoverURL(context.Background(), "URL")
		s.NoError(err)

		info, err := s.curator.GetURLInfo(context.Background(), "URL")
		s.NoError(err)
		s.Equal(sredis.StatusQueue, info.Status)

		queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
		s.NoError(err)
		s.Equal(int64(1), queueLen)

		indexLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
		s.NoError(err)
		s.Equal(int64(0), indexLen)

		workerQueueLen, err := s.rdb.LLen(context.Background(), sredis.WorkerKey("0")).Result()
		s.NoError(err)
		s.Equal(int64(0), workerQueueLen)
	})
}

func (s *RedisSuite) TestRecoverURLConcurrent() {
	urls := s.createURLs()
	total := len(urls) * len(urls[0])
	s.prepareProcessingQueue(urls)
	wCount := len(urls)

	var wg sync.WaitGroup

	wg.Add(wCount)
	for i := range wCount {
		go func(i int) {
			defer wg.Done()
			for _, url := range urls[i] {
				err := s.curator.RecoverURL(context.Background(), url)
				s.NoError(err)

				info, err := s.curator.GetURLInfo(context.Background(), url)
				s.NoError(err)
				s.Equal(sredis.StatusQueue, info.Status)
			}
		}(i)
	}
	wg.Wait()

	queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
	s.NoError(err)
	s.Equal(int64(total), queueLen)

	indexLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
	s.NoError(err)
	s.Equal(int64(0), indexLen)
}

func (s *RedisSuite) TestHealthCheck() {
	s.Run("Empty processing", func() {
		defer s.rdb.FlushDB(context.Background())

		err := s.curator.HealthCheck(context.Background(), time.Minute)
		s.NoError(err)

		queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
		s.NoError(err)
		s.Equal(int64(0), queueLen)

		hashLen, err := s.rdb.HLen(context.Background(), sredis.KeyURLStatus).Result()
		s.NoError(err)
		s.Equal(int64(0), hashLen)
	})

	s.Run("Orphaned URL", func() {
		const url = "ORPHAN"
		defer s.rdb.FlushDB(context.Background())

		s.rdb.SAdd(context.Background(), sredis.KeyProcessingIndex, url)

		err := s.curator.HealthCheck(context.Background(), time.Minute)
		s.NoError(err)

		queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
		s.NoError(err)
		s.Equal(int64(1), queueLen)

		indexLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
		s.NoError(err)
		s.Equal(int64(0), indexLen)

		hashLen, err := s.rdb.HLen(context.Background(), sredis.KeyURLStatus).Result()
		s.NoError(err)
		s.Equal(int64(1), hashLen)

		info, err := s.curator.GetURLInfo(context.Background(), url)
		if !s.NoError(err) {
			return
		}

		s.Equal(sredis.StatusQueue, info.Status)
	})

	s.Run("Less than threshold", func() {
		defer s.rdb.FlushDB(context.Background())

		urls := [][]string{{"URL"}}
		s.prepareProcessingQueue(urls)

		err := s.curator.HealthCheck(context.Background(), time.Hour)
		s.NoError(err)

		workerLen, err := s.rdb.LLen(context.Background(), sredis.WorkerKey("0")).Result()
		s.NoError(err)
		s.Equal(int64(1), workerLen)

		indexLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
		s.NoError(err)
		s.Equal(int64(1), indexLen)

		queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
		s.NoError(err)
		s.Equal(int64(0), queueLen)

		hashLen, err := s.rdb.HLen(context.Background(), sredis.KeyURLStatus).Result()
		s.NoError(err)
		s.Equal(int64(1), hashLen)

		info, err := s.curator.GetURLInfo(context.Background(), "URL")
		if !s.NoError(err) {
			return
		}

		s.Equal(sredis.StatusProcessing, info.Status)
	})

	s.Run("Greater than threshold", func() {
		defer s.rdb.FlushDB(context.Background())

		urls := [][]string{{"URL"}}
		s.prepareProcessingQueue(urls)

		time.Sleep(5 * time.Millisecond)
		err := s.curator.HealthCheck(context.Background(), time.Millisecond)
		s.NoError(err)

		workerLen, err := s.rdb.LLen(context.Background(), sredis.WorkerKey("0")).Result()
		s.NoError(err)
		s.Equal(int64(0), workerLen)

		indexLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
		s.NoError(err)
		s.Equal(int64(0), indexLen)

		queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
		s.NoError(err)
		s.Equal(int64(1), queueLen)

		hashLen, err := s.rdb.HLen(context.Background(), sredis.KeyURLStatus).Result()
		s.NoError(err)
		s.Equal(int64(1), hashLen)

		info, err := s.curator.GetURLInfo(context.Background(), "URL")
		if !s.NoError(err) {
			return
		}

		s.Equal(sredis.StatusQueue, info.Status)
	})

	s.Run("SScan pagination", func() {
		defer s.rdb.FlushDB(context.Background())
		urls := s.createURLs()
		s.prepareProcessingQueue(urls)
		total := len(urls) * len(urls[0])

		time.Sleep(10 * time.Millisecond)

		err := s.curator.HealthCheck(context.Background(), time.Millisecond)
		s.NoError(err)

		queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
		s.NoError(err)
		s.Equal(int64(total), queueLen)

		indexLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
		s.NoError(err)
		s.Equal(int64(0), indexLen)
	})
}
