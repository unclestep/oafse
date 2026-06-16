package parse_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	read "codeberg.org/readeck/go-readability/v2"
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
	pool           *pgxpool.Pool
	tx             pgx.Tx
	pageDS         *pg.PageDS
	urlDS          *sredis.URLDS
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
	s.pool = pool

	queue := sredis.NewQueue(rdb)
	retry := sredis.NewRetry(rdb)
	processing := sredis.NewProcessing(rdb, 1)
	s.urlDS = sredis.NewURLDS(rdb, queue, processing, retry)
}

func (s *ParseSuite) TearDownSuite() {
	s.pool.Close()
	s.Require().NoError(s.rdb.Close())
	s.Require().NoError(testcontainers.TerminateContainer(s.redisContainer))
	s.Require().NoError(testcontainers.TerminateContainer(s.pgContainer))
}

func (s *ParseSuite) SetupTest() {
	tx, err := s.pool.Begin(context.Background())
	s.Require().NoError(err)
	s.tx = tx
	s.pageDS = pg.NewPageDS(tx)
}

func (s *ParseSuite) TearDownTest() {
	s.Require().NoError(s.rdb.FlushDB(context.Background()).Err())
	s.Require().NoError(s.tx.Rollback(context.Background()))
}

func (s *ParseSuite) newUseCase(client *http.Client) *usecase.Parse {
	return s.newUseCaseWithConfig(client, &model.CrawlConfig{
		WorkersCount:              1,
		TryLim:                    3,
		TryBaseInterval:           100 * time.Millisecond,
		HealthCheckWorryThreshold: 15 * time.Second,
	})
}

func (s *ParseSuite) newUseCaseWithConfig(client *http.Client, cfg *model.CrawlConfig) *usecase.Parse {
	p := read.NewParser()
	p.CharThresholds = 0
	ext := extractor.NewExtractor(&p)
	fetch := fetcher.NewFetcher(client)
	proc := service.NewProcessing()

	pageRepo := repo.NewPageRepoDB(s.pageDS)
	urlRepo := repo.NewURLRepoCache(s.urlDS)
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
		<a href="/about">About</a>
		<a href="/topics">Topics</a>
		<a href="/page/2">Page 2</a>
		<a href="https://external.example.com">External site</a>
		<a href="https://another-external.org/path">Another external</a>
		</article>
		</body>
		</html>`))
	}))
	defer ts.Close()

	internalLinks := []string{
		ts.URL + "/about",
		ts.URL + "/topics",
		ts.URL + "/page/2",
	}

	ctx := context.Background()
	s.Require().NoError(s.urlDS.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd, "successful parse must return nil cmd")

	// Redis: source URL must be marked processed
	info, err := s.urlDS.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusProcessed, info.Status)

	// Redis: each discovered internal link must be enqueued for crawling
	for _, link := range internalLinks {
		linkInfo, err := s.urlDS.GetURLInfo(ctx, link)
		s.Require().NoError(err, "link %s must be present in Redis", link)
		s.Equal(sredis.StatusQueue, linkInfo.Status, "link %s must have queue status", link)
	}

	// Postgres: page saved with correct metadata
	page, err := s.pageDS.GetPage(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(ts.URL, page.URL)
	s.Equal("Web Crawlers", page.Title)
	s.NotEmpty(page.Content)

	// Postgres: links table contains exactly the internal links (extractor filters external)
	s.ElementsMatch(internalLinks, page.Links)
}

func (s *ParseSuite) TestExecuteEmptyQueueReturnsStop() {
	ctx := context.Background()
	cmd, err := s.newUseCase(http.DefaultClient).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Require().NotNil(cmd)
	s.Equal(port.DirectiveStop, cmd.Directive)
}

func (s *ParseSuite) TestExecuteServerErrorRetriesURL() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	ctx := context.Background()
	s.Require().NoError(s.urlDS.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	info, err := s.urlDS.GetURLInfo(ctx, ts.URL)
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
	s.Require().NoError(s.urlDS.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	info, err := s.urlDS.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusFailure, info.Status)
}

func (s *ParseSuite) TestExecuteRetryLimitExceededGivesUpURL() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	cfg := &model.CrawlConfig{
		WorkersCount:              1,
		TryLim:                    1,
		TryBaseInterval:           50 * time.Millisecond,
		HealthCheckWorryThreshold: 15 * time.Second,
	}
	uc := s.newUseCaseWithConfig(ts.Client(), cfg)

	ctx := context.Background()
	s.Require().NoError(s.urlDS.Start(ctx, ts.URL))

	cmd, err := uc.Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	info, err := s.urlDS.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusRetry, info.Status)
	s.Equal(1, info.Try)

	time.Sleep(150 * time.Millisecond)

	cmd, err = uc.Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	info, err = s.urlDS.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusFailure, info.Status)
}

func (s *ParseSuite) TestExecuteEmptyQueueWithProcessingReturnsSleep() {
	ctx := context.Background()
	s.Require().NoError(s.urlDS.Start(ctx, "http://example.com/page"))

	// Simulate another worker holding the URL in processing state
	_, err := s.urlDS.TakeOn(ctx, "other-worker")
	s.Require().NoError(err)

	// Queue is empty but there is a URL in processing -> DirectiveSleep
	cmd, err := s.newUseCase(http.DefaultClient).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Require().NotNil(cmd)
	s.Equal(port.DirectiveSleep, cmd.Directive)
	s.Positive(cmd.SleepFor)
}

func (s *ParseSuite) TestExecuteEmptyQueueWithRetryReturnsSleep() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer ts.Close()

	ctx := context.Background()
	s.Require().NoError(s.urlDS.Start(ctx, ts.URL))

	uc := s.newUseCase(ts.Client())

	// First execute: 503 -> URL moves to retry queue
	cmd, err := uc.Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	info, err := s.urlDS.GetURLInfo(ctx, ts.URL)
	s.Require().NoError(err)
	s.Equal(sredis.StatusRetry, info.Status)

	// Immediate second execute: queue empty, retry not ready yet -> DirectiveSleep
	// SleepFor reflects the time until EarliestRetry
	cmd, err = uc.Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Require().NotNil(cmd)
	s.Equal(port.DirectiveSleep, cmd.Directive)
	s.Positive(cmd.SleepFor)
}

func (s *ParseSuite) TestExecuteDuplicateLinksQueuedOnce() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>Duplicates Test</title></head>
		<body>
		<p>A page about web crawling with repeated links.</p>
		<a href="/about">About</a>
		<a href="/about">About again</a>
		<a href="/about">About third time</a>
		</body>
		</html>`))
	}))
	defer ts.Close()

	ctx := context.Background()
	s.Require().NoError(s.urlDS.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	// /about must be enqueued exactly once despite appearing three times in HTML
	linkInfo, err := s.urlDS.GetURLInfo(ctx, ts.URL+"/about")
	s.Require().NoError(err)
	s.Equal(sredis.StatusQueue, linkInfo.Status)

	// Total Redis entries: root URL (processed) + /about (queued) = 2
	count, err := s.rdb.HLen(ctx, sredis.KeyURLStatus).Result()
	s.Require().NoError(err)
	s.Equal(int64(2), count)
}

func (s *ParseSuite) TestExecuteExternalLinksNotQueued() {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`
		<!DOCTYPE html>
		<html>
		<head><title>External Links Test</title></head>
		<body>
		<p>A page with only external links.</p>
		<a href="https://external.example.com">External 1</a>
		<a href="https://another-external.org/path">External 2</a>
		</body>
		</html>`))
	}))
	defer ts.Close()

	ctx := context.Background()
	s.Require().NoError(s.urlDS.Start(ctx, ts.URL))

	cmd, err := s.newUseCase(ts.Client()).Execute(ctx, "worker-0")
	s.Require().NoError(err)
	s.Nil(cmd)

	// Only the root URL must be tracked in Redis - no external URLs
	count, err := s.rdb.HLen(ctx, sredis.KeyURLStatus).Result()
	s.Require().NoError(err)
	s.Equal(int64(1), count)

	_, err = s.urlDS.GetURLInfo(ctx, "https://external.example.com")
	s.Error(err, "external URL must be absent from Redis")
}

func TestParseSuite(t *testing.T) {
	suite.Run(t, new(ParseSuite))
}
