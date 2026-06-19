package di

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/application/usecase"
	deliveryhttp "oafse/internal/delivery/http"
	"oafse/internal/delivery/http/handler"
	"oafse/internal/delivery/http/middleware"
	"oafse/internal/delivery/worker"
	"oafse/internal/domain/model"
	"oafse/internal/domain/service"
	"oafse/internal/infrastructure/embedder"
	"oafse/internal/infrastructure/extractor"
	"oafse/internal/infrastructure/fetcher"
	"oafse/internal/infrastructure/storage/ds"
	postgresStorage "oafse/internal/infrastructure/storage/postgres"
	redisStorage "oafse/internal/infrastructure/storage/redis"
	"oafse/internal/infrastructure/storage/repo"

	read "codeberg.org/readeck/go-readability/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/fx"
)

func NewCrawler() fx.Option {
	return fx.Module(
		"crawler",
		fx.Provide(func() *model.CrawlConfig {
			return &model.CrawlConfig{
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
		crawlAppMod,
		workerMod,
		metricsMod,
		crawlControlMod,
		fx.Invoke(func(lc fx.Lifecycle, urlRepo port.URLRepo, cfg *model.CrawlConfig) {
			var cancel context.CancelFunc

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					appCtx, canc := context.WithCancel(context.Background())
					cancel = canc
					urlRepo.StartHealthChecking(appCtx, cfg)
					return nil
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
	fx.Provide(fx.Annotate(
		repo.NewPageRepoDB,
		fx.As(new(port.PageDBRepo)),
	)),
	fx.Provide(fx.Annotate(
		repo.NewURLRepoCache,
		fx.As(new(port.URLRepo)),
	)),
)

var dsMod = fx.Module(
	"ds",
	fx.Provide(func() (*pgxpool.Pool, error) {
		return postgresStorage.NewPool(os.Getenv("POSTGRES_DSN"))
	}),
	fx.Provide(func(pool *pgxpool.Pool) postgresStorage.DBTX {
		return pool
	}),
	fx.Invoke(func(lc fx.Lifecycle, pool *pgxpool.Pool) {
		var cancel context.CancelFunc

		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				appCtx, canc := context.WithCancel(context.Background())
				cancel = canc
				postgresStorage.StartPoolStatsPolling(appCtx, pool, 15*time.Second)
				return nil
			},
			OnStop: func(ctx context.Context) error {
				cancel()
				return nil
			},
		})
	}),
	fx.Provide(fx.Annotate(
		func() *postgresStorage.NotifyDS {
			return postgresStorage.NewNotifyDS(os.Getenv("POSTGRES_DSN"))
		},
		fx.As(new(ds.PageNotifyDS)),
	)),
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

var crawlAppMod = fx.Module(
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

var embedderMod = fx.Module(
	"embedder",
	fx.Provide(fx.Annotate(
		func() (*embedder.Embedder, error) {
			return embedder.NewEmbedder(os.Getenv("EMBEDDER_URL"))
		},
		fx.As(new(port.Embedder)),
	)),
)

var indexAppMod = fx.Module(
	"index",
	fx.Provide(fx.Annotate(
		usecase.NewIndex,
		fx.As(new(port.IndexUseCase)),
	)),
)

func NewIndexer() fx.Option {
	return fx.Module(
		"indexer",
		fx.Provide(func() *model.CrawlConfig {
			return &model.CrawlConfig{
				TryLim:          5,
				TryBaseInterval: 1 * time.Second,
				WorkersCount:    1,
			}
		}),
		domainMod,
		dsMod,
		repoMod,
		embedderMod,
		indexAppMod,
		metricsMod,
		fx.Invoke(func(lc fx.Lifecycle, embedder port.Embedder) {
			lc.Append(fx.Hook{
				OnStop: func(ctx context.Context) error {
					return embedder.Close()
				},
			})
		}),
		fx.Invoke(func(lc fx.Lifecycle, idx port.IndexUseCase, shutdowner fx.Shutdowner) {
			var cancel context.CancelFunc

			lc.Append(fx.Hook{
				OnStart: func(ctx context.Context) error {
					appCtx, canc := context.WithCancel(context.Background())
					cancel = canc

					go func() {
						idx.Execute(appCtx)
						if err := shutdowner.Shutdown(); err != nil {
							log.Printf("[ERROR] shutdown: %s", err)
						}
					}()

					return nil
				},
				OnStop: func(ctx context.Context) error {
					cancel()
					return nil
				},
			})
		}),
	)
}

var queryAppMod = fx.Module(
	"application",
	fx.Provide(fx.Annotate(
		usecase.NewQuery,
		fx.As(new(port.QueryUseCase)),
	)),
)

var httpMod = fx.Module(
	"http",
	fx.Provide(fx.Annotate(
		handler.NewQueryHandler,
		fx.As(new(http.Handler)),
		fx.ResultTags(`name:"queryHandler"`),
	)),
	fx.Provide(fx.Annotate(
		deliveryhttp.NewRouter,
		fx.ParamTags(`name:"queryHandler"`),
		fx.ResultTags(`name:"searchRouter"`),
	)),
	fx.Invoke(fx.Annotate(
		registerServer,
		fx.ParamTags("", `name:"searchRouter"`),
	)),
)

func registerServer(lc fx.Lifecycle, router *deliveryhttp.Router) {
	server := http.Server{
		Handler:      middleware.WithRecovery(router.Handler()),
		Addr:         os.Getenv("SERVER_ADDR"),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Printf("server error: %s", err)
				}
			}()
			log.Printf("\nserver has started on: %s", os.Getenv("SERVER_ADDR"))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}

func NewServer() fx.Option {
	return fx.Module(
		"server",
		dsMod,
		repoMod,
		embedderMod,
		queryAppMod,
		httpMod,
		metricsMod,
	)
}

var metricsMod = fx.Module(
	"metrics",
	fx.Invoke(registerMetricsServer),
)

var crawlControlMod = fx.Module(
	"crawl-control",
	fx.Provide(fx.Annotate(
		handler.NewCrawlHandler,
		fx.As(new(http.Handler)),
		fx.ResultTags(`name:"crawlHandler"`),
	)),
	fx.Provide(fx.Annotate(
		deliveryhttp.NewCrawlRouter,
		fx.ParamTags(`name:"crawlHandler"`),
		fx.ResultTags(`name:"crawlRouter"`),
	)),
	fx.Invoke(fx.Annotate(
		registerCrawlControlServer,
		fx.ParamTags("", `name:"crawlRouter"`),
	)),
)

func registerCrawlControlServer(lc fx.Lifecycle, router *deliveryhttp.Router) {
	server := http.Server{
		Handler: middleware.WithRecovery(router.Handler()),
		Addr:    os.Getenv("CRAWLER_ADDR"),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Printf("crawl control server error: %s", err)
				}
			}()
			log.Printf("crawl control server has started on: %s", os.Getenv("CRAWLER_ADDR"))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}

func registerMetricsServer(lc fx.Lifecycle) {
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())

	server := http.Server{
		Handler: mux,
		Addr:    os.Getenv("METRICS_ADDR"),
	}

	lc.Append(fx.Hook{
		OnStart: func(ctx context.Context) error {
			go func() {
				if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
					log.Printf("metrics server error: %s", err)
				}
			}()
			log.Printf("metrics server has started on: %s", os.Getenv("METRICS_ADDR"))
			return nil
		},
		OnStop: func(ctx context.Context) error {
			return server.Shutdown(ctx)
		},
	})
}
