package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

type NotifyDS struct {
	dsn string
}

func NewNotifyDS(dsn string) *NotifyDS {
	return &NotifyDS{
		dsn: dsn,
	}
}

func (s *NotifyDS) sleepWithContext(ctx context.Context, delay time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(delay):
	}
}

func (s *NotifyDS) StartListening(ctx context.Context, music string) (chan bool, chan error) {
	hasWork := make(chan bool, 1)
	errCh := make(chan error, 1)
	query := fmt.Sprintf(`LISTEN %s`, pgx.Identifier{music}.Sanitize())

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			conn, err := pgx.Connect(ctx, s.dsn)
			if err != nil {
				select {
				case errCh <- fmt.Errorf("start listening: %w", err):
				case <-ctx.Done():
					return
				default:
				}
				s.sleepWithContext(ctx, 5*time.Second)
				continue
			}

			if _, err := conn.Exec(ctx, query); err != nil {
				_ = conn.Close(ctx)
				select {
				case errCh <- fmt.Errorf("start listening: %w", err):
				case <-ctx.Done():
					return
				default:
				}
				continue
			}

			for {
				_, err = conn.WaitForNotification(ctx)
				if err != nil {
					_ = conn.Close(ctx)
					select {
					case errCh <- fmt.Errorf("start listening: %w", err):
					case <-ctx.Done():
						return
					default:
					}
					break
				}

				select {
				case hasWork <- true:
				case <-ctx.Done():
					_ = conn.Close(ctx)
					return
				default:
				}
			}

		}
	}()

	return hasWork, errCh
}
