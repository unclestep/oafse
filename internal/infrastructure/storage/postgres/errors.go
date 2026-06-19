package postgres

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"oafse/internal/application/port"
	"oafse/internal/infrastructure/metrics"
)

func observeQuery(query string) func() {
	start := time.Now()
	return func() {
		metrics.PostgresQueryDuration.WithLabelValues(query).Observe(time.Since(start).Seconds())
	}
}

func isTransientErr(err error) bool {
	if _, ok := errors.AsType[*pgconn.PgError](err); ok {
		return false
	}

	if pgconn.SafeToRetry(err) {
		return true
	}

	if _, ok := errors.AsType[net.Error](err); ok {
		return true
	}

	return errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

func wrapDBErr(op string, err error) error {
	if isTransientErr(err) {
		metrics.PostgresErrors.WithLabelValues("transient").Inc()
		return fmt.Errorf("%s: %w: %w", op, port.ErrTransient, err)
	}
	metrics.PostgresErrors.WithLabelValues("fatal").Inc()
	return fmt.Errorf("%s: %w", op, err)
}
