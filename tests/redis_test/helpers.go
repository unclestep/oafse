package redis_test

import (
	"context"
	"strings"

	sredis "oafse/internal/redis"
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
	var alpha = int(byte('Z')-byte('A')) + 1
	urls := make([][]string, alpha)

	for i := range alpha {
		for j := 1; j <= 3; j++ {
			var ch = byte('A') + byte(i)
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
