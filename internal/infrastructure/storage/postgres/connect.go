package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPool(databaseURL string) (*pgxpool.Pool, error) {
	conf, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("pgx parse config: %w", err)
	}

	conf.MaxConnLifetime = 15 * time.Minute
	conf.MaxConnLifetimeJitter = 15 * time.Minute
	conf.MaxConnIdleTime = 5 * time.Minute
	conf.MaxConns = 20
	conf.MinConns = 4

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, conf)
	if err != nil {
		return nil, fmt.Errorf("pgxpool creation: %w", err)
	}

	err = pool.Ping(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("pgxpool ping: %w", err)
	}

	return pool, nil
}
