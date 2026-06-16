package postgres

import (
	"context"
	"fmt"

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

func (s *NotifyDS) WaitForNotification(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, s.dsn)
	if err != nil {
		return fmt.Errorf("wait for notification: %w", err)
	}
	defer func() {
		_ = conn.Close(ctx)
	}()

	if _, err := conn.Exec(ctx, "LISTEN page_inserted"); err != nil {
		return fmt.Errorf("wait for notification: %w", err)
	}

	_, err = conn.WaitForNotification(ctx)
	return err
}
