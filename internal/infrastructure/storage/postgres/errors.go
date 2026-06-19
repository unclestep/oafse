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

var transientPgErrorCodes = map[string]bool{
	"40001": true, // serialization_failure
	"40P01": true, // deadlock_detected
	"53300": true, // too_many_connections
	"57P03": true, // cannot_connect_now
}

func isTransientErr(err error) bool {
	if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
		return transientPgErrorCodes[pgErr.Code]
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
