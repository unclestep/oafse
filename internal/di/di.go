package di

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/application/usecase"
	"oafse/internal/delivery/worker"
	"oafse/internal/domain/model"
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

func NewCrawler(startURL string, resume bool) fx.Option {
	return fx.Module(
		"crawler",
		fx.Provide(func() *model.CrawlConfig {
			return &model.CrawlConfig{
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
		fx.Invoke(func(lc fx.Lifecycle, repo port.CrawlRepo, cfg *model.CrawlConfig) {
			var cancel context.CancelFunc

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					appCtx, canc := context.WithCancel(context.Background())
					cancel = canc

					if !resume {
						if err := repo.ResetCrawlCache(ctx); err != nil {
							return err
						}
					}

					repo.StartHealthChecking(appCtx, cfg)
					return repo.Start(ctx, cfg.StartURL)
				},
				OnStop: func(ctx context.Context) error {
					cancel()
					return nil
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, pool *worker.WorkerPool, fetcher port.Fetcher, shutdowner fx.Shutdowner) {
			var cancel context.CancelFunc

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					appCtx, canc := context.WithCancel(context.Background())
					cancel = canc

					go func() {
						pool.Run(appCtx)
						if err := shutdowner.Shutdown(); err != nil {
							log.Printf("[ERROR] shutdown: %s", err)
						}
					}()

					return nil
				},
				OnStop: func(ctx context.Context) error {
					cancel()
					fetcher.CloseBrowser()
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
			Timeout: 30 * time.Second,
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
	fx.Provide(func(rdb *goredis.Client, cfg *model.CrawlConfig) *redisStorage.Processing {
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
