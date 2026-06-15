package di

import (
	"context"
	"net/http"
	"os"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/application/usecase"
	"oafse/internal/delivery/worker"
	"oafse/internal/domain/service"
	"oafse/internal/infrastructure/extractor"
	"oafse/internal/infrastructure/fetcher"
	"oafse/internal/infrastructure/storage/ds"
	postgresStorage "oafse/internal/infrastructure/storage/postgres"
	redisStorage "oafse/internal/infrastructure/storage/redis"
	"oafse/internal/infrastructure/storage/repo"

	read "codeberg.org/readeck/go-readability/v2"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func NewCrawler(startURL string) fx.Option {
	return fx.Module(
		"crawler",
		fx.Provide(func() *port.CrawlConfig {
			return &port.CrawlConfig{
				StartURL:                  startURL,
				WorkersCount:              20,
				TryLim:                    5,
				TryBaseInterval:           5 * time.Second,
				HealthCheckWorryThreshold: 15 * time.Second,
			}
		}),
		fetcherMod,
		extractorMod,
		domainMod,
		repoMod,
		dsMod,
		appMod,
		workerMod,
		fx.Invoke(func(lc fx.Lifecycle, repo port.CrawlRepo, cfg *port.CrawlConfig) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					repo.StartHealthChecking(context.Background(), cfg)
					return repo.Start(ctx, cfg.StartURL)
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, pool *worker.WorkerPool) {
			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					go pool.Run(context.Background())
					return nil
				},
			})
		}),
	)
}

var fetcherMod = fx.Module(
	"fetcher",
	fx.Provide(func() *http.Client {
		return &http.Client{
			Timeout: 15 * time.Second,
		}
	}),
	fx.Provide(fx.Annotate(
		fetcher.NewFetcher,
		fx.As(new(port.Fetcher)),
	)),
)

var extractorMod = fx.Module(
	"extractor",
	fx.Provide(func() *read.Parser {
		p := read.NewParser()
		p.CharThresholds = 0
		return &p
	}),
	fx.Provide(fx.Annotate(
		extractor.NewExtractor,
		fx.As(new(port.Extractor)),
	)),
)

var domainMod = fx.Module(
	"domain",
	fx.Provide(fx.Annotate(
		service.NewProcessing,
		fx.As(new(port.Processing)),
	)),
)

var repoMod = fx.Module(
	"repo",
	fx.Provide(repo.NewPageRepoDB),
	fx.Provide(repo.NewURLRepoCache),
	fx.Provide(fx.Annotate(
		repo.NewCrawlRepo,
		fx.As(new(port.CrawlRepo)),
	)),
)

var dsMod = fx.Module(
	"ds",
	fx.Provide(func() (postgresStorage.DBTX, error) {
		return postgresStorage.NewPool(os.Getenv("POSTGRES_DSN"))
	}),
	fx.Provide(fx.Annotate(
		postgresStorage.NewPageDS,
		fx.As(new(ds.PageDBDS)),
	)),
	fx.Provide(func() (*goredis.Client, error) {
		return redisStorage.NewClient(os.Getenv("REDIS_DSN"))
	}),
	fx.Provide(redisStorage.NewQueue),
	fx.Provide(func(rdb *goredis.Client, cfg *port.CrawlConfig) *redisStorage.Processing {
		return redisStorage.NewProcessing(rdb, cfg.WorkersCount)
	}),
	fx.Provide(redisStorage.NewRetry),
	fx.Provide(fx.Annotate(
		redisStorage.NewURLDS,
		fx.As(new(ds.URLCacheDS)),
	)),
)

var appMod = fx.Module(
	"application",
	fx.Provide(fx.Annotate(
		usecase.NewParse,
		fx.As(new(port.ParseUseCase)),
	)),
)

var workerMod = fx.Module(
	"worker",
	fx.Provide(worker.NewWorkerPool),
)
