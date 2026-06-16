package redis_test

import (
	"context"
	"testing"

	sredis "oafse/internal/infrastructure/storage/redis"

	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

const (
	WorkersCount = 20
)

type RedisSuite struct {
	suite.Suite
	container *tcredis.RedisContainer
	rdb       *redis.Client
	curator   *sredis.URLDS
}

func (s *RedisSuite) SetupSuite() {
	ctx := context.Background()

	container, err := tcredis.Run(
		ctx, "redis:alpine",
	)

	s.Require().NoError(err)
	s.container = container

	databaseURL, err := container.ConnectionString(ctx)
	s.Require().NoError(err)

	rdb, err := sredis.NewClient(databaseURL)
	s.Require().NoError(err)
	s.rdb = rdb

	queue := sredis.NewQueue(rdb)
	retry := sredis.NewRetry(rdb)
	processing := sredis.NewProcessing(rdb, WorkersCount)
	s.curator = sredis.NewURLDS(rdb, queue, processing, retry)
}

func (s *RedisSuite) TearDownSuite() {
	err := s.rdb.Close()
	s.Require().NoError(err)

	err = testcontainers.TerminateContainer(s.container)
	s.Require().NoError(err)
}

func (s *RedisSuite) TearDownTest() {
	err := s.rdb.FlushDB(context.Background()).Err()
	s.Require().NoError(err)
}

func TestRedisSuite(t *testing.T) {
	suite.Run(t, new(RedisSuite))
}
