package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type Page struct {
	ID          int64     `json:"id"`
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Body        string    `json:"body"`
	Status      int16     `json:"status"`
	CrawledAt   time.Time `json:"crawled_at"`
}

type Link struct {
	SrcPageID int64  `json:"src_page_id"`
	DstURL    string `json:"dst_url"`
}

type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, arguments ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, arguments ...any) pgx.Row
}

type TxBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

func InsertPage(ctx context.Context, db DBTX, page *Page) (int64, error) {
	sql := `
		INSERT INTO pages (url, title, description, body, status, crawled_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (url) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			body = EXCLUDED.body,
			status = EXCLUDED.status,
			crawled_at = EXCLUDED.crawled_at
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var pageID int64

	err := db.QueryRow(ctx, sql, page.URL, page.Title, page.Description, page.Body, page.Status, page.CrawledAt).Scan(&pageID)
	if err != nil {
		return -1, fmt.Errorf("insert page: %w", err)
	}

	return pageID, nil
}

func InsertLink(ctx context.Context, db DBTX, link *Link) error {
	sql := `
		INSERT INTO links (src_page_id, dst_url)
		VALUES ($1, $2)
		ON CONFLICT (src_page_id, dst_url) DO NOTHING
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err := db.Exec(ctx, sql, link.SrcPageID, link.DstURL)
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}

	return nil
}

func GetPage(ctx context.Context, db DBTX, pageURL string) (*Page, error) {
	sql := `
		SELECT id, url, title, description, body, status, crawled_at
		FROM pages
		WHERE url = $1
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var page Page
	err := db.QueryRow(ctx, sql, pageURL).Scan(
		&page.ID,
		&page.URL,
		&page.Title,
		&page.Description,
		&page.Body,
		&page.Status,
		&page.CrawledAt,
	)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	return &page, nil
}

func PageExists(ctx context.Context, db DBTX, pageURL string) (bool, error) {
	sql := `
		SELECT EXISTS(
			SELECT 1
			FROM pages
			WHERE url = $1
		)
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var exists bool
	err := db.QueryRow(ctx, sql, pageURL).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("page exists: %w", err)
	}

	return exists, nil
}

func SavePageCrawl(ctx context.Context, txb TxBeginner, page *Page, links []*Link) error {
	tx, err := txb.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	defer tx.Rollback(ctx) //nolint:errcheck

	_, err = InsertPage(ctx, tx, page)
	if err != nil {
		return err
	}

	for _, link := range links {
		if err := InsertLink(ctx, tx, link); err != nil {
			return err
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
