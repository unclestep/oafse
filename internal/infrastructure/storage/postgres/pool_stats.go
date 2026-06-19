package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"oafse/internal/infrastructure/metrics"
)

func StartPoolStatsPolling(ctx context.Context, pool *pgxpool.Pool, interval time.Duration) {
	go func() {
		var lastAcquireCount, lastEmptyAcquireCount, lastCanceledAcquireCount int64
		var lastAcquireDuration, lastEmptyAcquireWaitTime time.Duration

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				stat := pool.Stat()

				metrics.PostgresPoolAcquiredConns.Set(float64(stat.AcquiredConns()))
				metrics.PostgresPoolIdleConns.Set(float64(stat.IdleConns()))
				metrics.PostgresPoolTotalConns.Set(float64(stat.TotalConns()))
				metrics.PostgresPoolMaxConns.Set(float64(stat.MaxConns()))

				metrics.PostgresPoolAcquireCount.Add(float64(stat.AcquireCount() - lastAcquireCount))
				lastAcquireCount = stat.AcquireCount()

				metrics.PostgresPoolEmptyAcquireCount.Add(float64(stat.EmptyAcquireCount() - lastEmptyAcquireCount))
				lastEmptyAcquireCount = stat.EmptyAcquireCount()

				metrics.PostgresPoolCanceledAcquireCount.Add(float64(stat.CanceledAcquireCount() - lastCanceledAcquireCount))
				lastCanceledAcquireCount = stat.CanceledAcquireCount()

				metrics.PostgresPoolAcquireDuration.Add((stat.AcquireDuration() - lastAcquireDuration).Seconds())
				lastAcquireDuration = stat.AcquireDuration()

				metrics.PostgresPoolEmptyAcquireWaitTime.Add((stat.EmptyAcquireWaitTime() - lastEmptyAcquireWaitTime).Seconds())
				lastEmptyAcquireWaitTime = stat.EmptyAcquireWaitTime()
			case <-ctx.Done():
				return
			}
		}
	}()
}
