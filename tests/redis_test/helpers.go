package redis_test

import (
	"context"
	"fmt"
	"strings"

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

func (s *RedisSuite) pushURLs() [][]string {
	urls := s.createURLs()
	for _, group := range urls {
		pushed, err := s.curator.PushURLs(context.Background(), group)
		s.NoError(err)
		s.ElementsMatch(group, pushed)
		s.Equal(len(group), len(pushed))
	}
	return urls
}

func (s *RedisSuite) prepareProcessingQueue(urls [][]string) {
	for i, group := range urls {
		pushed, err := s.curator.PushURLs(context.Background(), group)
		s.NoError(err)
		s.Equal(len(group), len(pushed))

		var allPopped []string
		for {
			popped, err := s.curator.PopURL(context.Background(), fmt.Sprint(i))
			if !s.NoError(err) {
				return
			}

			if popped == "" {
				break
			}

			allPopped = append(allPopped, popped)
		}

		s.ElementsMatch(group, allPopped)
	}
}
