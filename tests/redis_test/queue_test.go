package redis_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	sredis "oafse/internal/redis"
)

func (s *RedisSuite) TestQueuePushURL() {
	urls := []string{
		"URL1", "URL2", "URL3", "URL4",
	}

	for _, url := range urls {
		subtestName := fmt.Sprintf("Queue push URL: %s", url)
		s.Run(subtestName, func() {
			ok, err := s.curator.PushURL(context.Background(), url)
			s.NoError(err)
			s.True(ok)

			info, err := s.curator.GetURLInfo(context.Background(), url)
			s.NoError(err)
			s.Equal(sredis.StatusQueue, info.Status)
		})
	}

	s.checkQueue(len(urls), len(urls))

	for _, url := range urls {
		subtestName := fmt.Sprintf("Queue push same URL: %s", url)
		s.Run(subtestName, func() {
			ok, err := s.curator.PushURL(context.Background(), url)
			s.NoError(err)
			s.False(ok)
		})
	}

	s.checkQueue(len(urls), len(urls))
}

func (s *RedisSuite) TestQueuePushURLConcurrent() {
	url := "URL"
	var okPushed atomic.Int32

	var wg sync.WaitGroup
	wg.Add(WorkersCount)

	for range WorkersCount {
		go func() {
			defer wg.Done()
			ok, err := s.curator.PushURL(context.Background(), url)
			s.NoError(err)
			if ok {
				okPushed.Add(1)
			}
		}()
	}
	wg.Wait()

	s.Equal(int32(1), okPushed.Load())
	s.checkQueue(1, 1)
}

func (s *RedisSuite) TestQueuePushURLsConcurrent() {
	urls := s.createURLs()
	g := len(urls)

	var wg sync.WaitGroup
	wg.Add(g)

	for i := range g {
		go func(i int) {
			defer wg.Done()
			pushed, err := s.curator.PushURLs(context.Background(), urls[i])
			if !s.NoError(err) {
				return
			}

			if pushed == nil {
				return
			}

			s.Equal(len(urls[i]), len(pushed))
			s.ElementsMatch(urls[i], pushed)
		}(i)
	}
	wg.Wait()

	s.checkQueue(len(urls)*3, len(urls)*3)
}

func (s *RedisSuite) TestPopURLConcurrent() {
	urls := s.pushURLs()
	total := len(urls) * len(urls[0])

	var wg sync.WaitGroup

	wg.Add(WorkersCount)
	for range WorkersCount {
		go func() {
			defer wg.Done()
			workerID := fmt.Sprint(rand.Int31n(int32(WorkersCount)))
			for {
				url, err := s.curator.PopURL(context.Background(), workerID)
				if !s.NoError(err) {
					return
				}
				if url == "" {
					break
				}

				info, err := s.curator.GetURLInfo(context.Background(), url)
				if !s.NoError(err) {
					return
				}

				s.Equal(sredis.StatusProcessing, info.Status, "Workers Processing Queue")
				s.Equal(workerID, info.WorkerID, "Workers ID")
				s.Greater(info.ProcessingStartTime, time.Now().UnixMilli()-time.Minute.Nanoseconds(), "Processing Start Time")
			}
		}()
	}
	wg.Wait()

	s.checkQueue(0, total)
	indexLen, err := s.rdb.SCard(context.Background(), sredis.KeyProcessingIndex).Result()
	s.NoError(err)
	s.Equal(int64(total), indexLen, "Global processing index")
}
