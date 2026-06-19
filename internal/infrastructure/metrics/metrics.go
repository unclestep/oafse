package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	StatusOK         = "ok"
	StatusRetry      = "retry"
	StatusImpossible = "impossible"
	StatusGiveUp     = "give_up"
	StatusManual     = "manual"
)

var (
	// Crawler
	FetchAttempts = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "crawler_fetch_attempts_total",
		Help: "Total fetch attempts, partitioned by outcome status.",
	}, []string{"status"}) // ok | retry | impossible | give_up | manual

	FetchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "crawler_fetch_duration_seconds",
		Help:    "HTTP fetch latency, partitioned by fetch method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"via"}) // http | chromedp

	FetchErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "crawler_fetch_errors_total",
		Help: "Total crawl errors partitioned by reason.",
	}, []string{"reason"}) // timeout, dns_error, http_5xx, parse_error

	BytesFetched = promauto.NewCounter(prometheus.CounterOpts{
		Name: "crawler_bytes_fetched_total",
		Help: "Total bytes downloaded by the crawler.",
	})

	SPAFallbacks = promauto.NewCounter(prometheus.CounterOpts{
		Name: "crawler_spa_fallback_total",
		Help: "Pages that triggered chromedp fallback due to SPA markers.",
	})

	QueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "crawler_queue_depth",
		Help: "Current number of URLs in the main crawl queue.",
	})

	RetryQueueDepth = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "crawler_retry_queue_depth",
		Help: "Current number of URLs in the retry queue.",
	})

	ActiveWorkers = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "crawler_active_workers",
		Help: "Number of crawler workers currently active.",
	})

	// Indexer
	IndexerBacklog = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_unvectorized_backlog",
		Help: "Pages crawled but not yet embedded.",
	})

	IndexerProgress = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "indexer_pages_in_progress",
		Help: "Pages currently being embedded.",
	})

	EmbedDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "indexer_embed_duration_seconds",
		Help:    "Embedding call latency, partitioned by call method.",
		Buckets: prometheus.DefBuckets,
	}, []string{"method"}) // single | batch

	PagesEmbedded = promauto.NewCounter(prometheus.CounterOpts{
		Name: "indexer_pages_embedded_total",
		Help: "Total pages successfully vectorized.",
	})

	EmbedErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "indexer_embed_errors_total",
		Help: "Total embedding generation errors partitioned by reason.",
	}, []string{"reason"}) // connection_error, too_big_text

	// Search server
	SearchDuration = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "search_request_duration_seconds",
		Help:    "End-to-end search request latency.",
		Buckets: prometheus.DefBuckets,
	})

	SearchRequests = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "search_requests_total",
		Help: "Total search requests partitioned by HTTP status.",
	}, []string{"status"}) // 200 | 400 | 500

	EmptySearchResults = promauto.NewCounter(prometheus.CounterOpts{
		Name: "search_empty_results_total",
		Help: "Total search queries that returned zero results.",
	})

	// Postgres
	PostgresQueryDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "postgres_query_duration_seconds",
		Help:    "Postgres query latency, partitioned by query.",
		Buckets: prometheus.DefBuckets,
	}, []string{"query"}) // get_page | page_exists | insert_page | insert_link | get_unvectorized | find_similar

	PostgresErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "postgres_errors_total",
		Help: "Total postgres errors partitioned by reason.",
	}, []string{"reason"}) // transient | fatal

	PostgresPoolAcquiredConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "postgres_pool_acquired_conns",
		Help: "Connections currently acquired from the pool.",
	})

	PostgresPoolIdleConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "postgres_pool_idle_conns",
		Help: "Idle connections currently in the pool.",
	})

	PostgresPoolTotalConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "postgres_pool_total_conns",
		Help: "Total connections currently in the pool (acquired + idle + constructing).",
	})

	PostgresPoolMaxConns = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "postgres_pool_max_conns",
		Help: "Maximum size the pool can reach.",
	})

	PostgresPoolAcquireCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "postgres_pool_acquire_total",
		Help: "Total successful connection acquires from the pool.",
	})

	PostgresPoolAcquireDuration = promauto.NewCounter(prometheus.CounterOpts{
		Name: "postgres_pool_acquire_duration_seconds_total",
		Help: "Cumulative time spent waiting to acquire a connection from the pool.",
	})

	PostgresPoolEmptyAcquireCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "postgres_pool_empty_acquire_total",
		Help: "Total acquires that had to wait because the pool had no idle connection.",
	})

	PostgresPoolEmptyAcquireWaitTime = promauto.NewCounter(prometheus.CounterOpts{
		Name: "postgres_pool_empty_acquire_wait_seconds_total",
		Help: "Cumulative wait time for acquires that had to wait because the pool had no idle connection.",
	})

	PostgresPoolCanceledAcquireCount = promauto.NewCounter(prometheus.CounterOpts{
		Name: "postgres_pool_canceled_acquire_total",
		Help: "Total acquires canceled by their context before a connection was obtained.",
	})
)
