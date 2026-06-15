package parse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	read "codeberg.org/readeck/go-readability/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"

	"oafse/internal/application/port"
	"oafse/internal/application/usecase"
	"oafse/internal/domain/model"
	"oafse/internal/domain/service"
	"oafse/internal/infrastructure/extractor"
	"oafse/internal/infrastructure/fetcher"
	pg "oafse/internal/infrastructure/storage/postgres"
	sredis "oafse/internal/infrastructure/storage/redis"
	"oafse/internal/infrastructure/storage/repo"
)

type ParseSuite struct {
	suite.Suite
	redisContainer *tcredis.RedisContainer
	pgContainer    *tcpostgres.PostgresContainer
	rdb            *redis.Client
	pool           *pg.DBTX
	curator        *sredis.URLDS
}

func (s *ParseSuite) SetupSuite() {
	ctx := context.Background()

	redisContainer, err := tcredis.Run(ctx, "redis:alpine")
	s.Require().NoError(err)
	s.redisContainer = redisContainer

	redisDSN, err := redisContainer.ConnectionString(ctx)
	s.Require().NoError(err)

	rdb, err := sredis.NewClient(redisDSN)
	s.Require().NoError(err)
	s.rdb = rdb

	pgContainer, err := tcpostgres.Run(ctx,
		"postgres:18-alpine",
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("dbuser"),
		tcpostgres.WithPassword("dbpassword"),
		tcpostgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err)
	s.pgContainer = pgContainer

	pgDSN, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	s.Require().NoError(err)

	s.Require().NoError(pg.Migrate(pgDSN))

	pool, err := pg.NewPool(pgDSN)
	s.Require().NoError(err)

	dbtx := pg.DBTX(pool)
	s.pool = &dbtx

	queue := sredis.NewQueue(rdb)
	retry := sredis.NewRetry(rdb)
	processing := sredis.NewProcessing(rdb, 1)
	s.curator = sredis.NewURLDS(rdb, queue, processing, retry)
}

func (s *ParseSuite) TearDownSuite() {
	s.Require().NoError(s.rdb.Close())
	s.Require().NoError(testcontainers.TerminateContainer(s.redisContainer))
	s.Require().NoError(testcontainers.TerminateContainer(s.pgContainer))
}

func (s *ParseSuite) TearDownTest() {
	s.Require().NoError(s.rdb.FlushDB(context.Background()).Err())
}

func (s *ParseSuite) newUseCase(client *http.Client) *usecase.Parse {
	cfg := &model.CrawlConfig{
		WorkersCount:              1,
		TryLim:                    3,
		TryBaseInterval:           100 * time.Millisecond,
		HealthCheckWorryThreshold: 15 * time.Second,
	}

	p := read.NewParser()
	p.CharThresholds = 0
	ext := extractor.NewExtractor(&p)
	fetch := fetcher.NewFetcher(client)
	proc := service.NewProcessing()

	pageDS := pg.NewPageDS(*s.pool)
	pageRepo := repo.NewPageRepoDB(pageDS)
	urlRepo := repo.NewURLRepoCache(s.curator)
	crawlRepo := repo.NewCrawlRepo(urlRepo, pageRepo)

	return usecase.NewParse(cfg, crawlRepo, proc, fetch, ext)
}

func (s *ParseSuite) TestExecuteParsesPageAndStoresResult() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head>
		<title>Web Crawlers</title>
		<meta name="description" content="An introduction to web crawlers">
		</head>
		<body>
		<article>
		<h1>Web Crawlers</h1>
		<p>A web crawler is an automated program that systematically browses the internet
		to collect data from web pages. Crawlers are used by search engines to index content,
		by researchers to analyse web structure, and by data pipelines to aggregate information
		from multiple sources. They follow hyperlinks to discover new pages and store the
		extracted content in a database for later retrieval and analysis.</p>
		<a href="/about">About (internal)</a>
		<a href="https://external.example.com">External (filtered)</a>
		</article>
		</body>
		</html>`))
	}))
	defer ts.Close()

	ctx := context.Background()
	s.Require().NoError(s.curator.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd, "successful parse must return nil cmd")

	// Redis: URL must be marked processed
	info, err := s.curator.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusProcessed, info.Status)

	// Postgres: page must be saved with correct fields
	ds := pg.NewPageDS(*s.pool)
	page, err := ds.GetPage(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(ts.URL, page.URL)
	s.Equal("Web Crawlers", page.Title)
	s.NotEmpty(page.Content)
	s.Contains(page.Links, ts.URL+"/about")
	s.NotContains(page.Links, "https://external.example.com")
}

func (s *ParseSuite) TestExecuteEmptyQueue_ReturnsStop() {
	ctx := context.Background()
	cmd, err := s.newUseCase(http.DefaultClient).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Require().NotNil(cmd)
	s.Equal(port.DirectiveStop, cmd.Directive)
}

func (s *ParseSuite) TestExecuteServerError_RetriesURL() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	ctx := context.Background()
	s.Require().NoError(s.curator.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	info, err := s.curator.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusRetry, info.Status)
	s.Equal(1, info.Try)
}

func (s *ParseSuite) TestExecutePermanentErrorGivesUpURL() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx := context.Background()
	s.Require().NoError(s.curator.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	info, err := s.curator.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusFailure, info.Status)
}

func TestParseSuite(t *testing.T) {
	suite.Run(t, new(ParseSuite))
}
