package redis_test

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"oafse/internal/application/port"
	storage "oafse/internal/infrastructure/storage/model"
	sredis "oafse/internal/infrastructure/storage/redis"
)

func (s *RedisSuite) checkQueue(expQueueLen, expMetaLen int) {
	queueLen, err := s.rdb.LLen(context.Background(), sredis.KeyQueue).Result()
	s.NoError(err)
	s.Equal(int64(expQueueLen), queueLen)

	statusCount, err := s.rdb.HLen(context.Background(), sredis.KeyURLStatus).Result()
	s.NoError(err)
	s.Equal(int64(expMetaLen), statusCount)
}

func (s *RedisSuite) createURLs() [][]string {
	alpha := int(byte('Z')-byte('A')) + 1
	urls := make([][]string, alpha)

	for i := range alpha {
		for j := 1; j <= 3; j++ {
			ch := byte('A') + byte(i)
			urls[i] = append(urls[i], strings.Repeat(string(ch), j))
		}
	}

	return urls
}

func toURLCaches(urls []string) []*storage.URLCache {
	caches := make([]*storage.URLCache, len(urls))
	for i, u := range urls {
		caches[i] = &storage.URLCache{URL: u}
	}
	return caches
}

func urlCacheURLs(caches []*storage.URLCache) []string {
	urls := make([]string, len(caches))
	for i, c := range caches {
		urls[i] = c.URL
	}
	return urls
}

func (s *RedisSuite) pushURLs() [][]string {
	urls := s.createURLs()
	for _, group := range urls {
		pushed, err := s.curator.PushURLs(context.Background(), toURLCaches(group))
		s.NoError(err)
		s.ElementsMatch(group, urlCacheURLs(pushed))
		s.Equal(len(group), len(pushed))
	}
	return urls
}

func (s *RedisSuite) prepareProcessingQueue(urls [][]string) {
	for i, group := range urls {
		pushed, err := s.curator.PushURLs(context.Background(), toURLCaches(group))
		s.NoError(err)
		s.Equal(len(group), len(pushed))

		var allPopped []string
		for {
			cache, err := s.curator.TakeOn(context.Background(), fmt.Sprint(i))
			if errors.Is(err, port.ErrQueueEmpty) {
				break
			}
			if !s.NoError(err) {
				return
			}
			allPopped = append(allPopped, cache.URL)
		}

		s.ElementsMatch(group, allPopped)
	}
}
