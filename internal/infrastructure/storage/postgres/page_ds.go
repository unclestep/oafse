package postgres

import (
	"context"
	"fmt"
	"time"

	"oafse/internal/application/port"
	storage "oafse/internal/infrastructure/storage/model"

	"github.com/jackc/pgx/v5"
)

type link struct {
	srcPageID int64
	dstURL    string
}

type PageDS struct {
	dbtx DBTX
}

func NewPageDS(dbtx DBTX) *PageDS {
	return &PageDS{
		dbtx: dbtx,
	}
}

func (s *PageDS) GetPage(ctx context.Context, url string) (*storage.PageDB, error) {
	sql := `
		SELECT p.url, p.title, p.description, p.content, p.crawled_at, l.dst_url
		FROM pages p
		LEFT JOIN links l ON l.src_page_id = p.id
		WHERE url = $1
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var page storage.PageDB
	var title, desc, content *string
	var crawledAt *time.Time
	rows, err := s.dbtx.Query(ctx, sql, url)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}
	defer rows.Close()

	seen := false
	for rows.Next() {
		var dstURL *string
		err := rows.Scan(&page.URL, &title, &desc, &content, &crawledAt, &dstURL)
		if err != nil {
			return nil, fmt.Errorf("get page: %w", err)
		}

		if !seen {
			if title != nil {
				page.Title = *title
			}
			if desc != nil {
				page.Description = *desc
			}
			if content != nil {
				page.Content = *content
			}
			if crawledAt != nil {
				page.CrawledAt = *crawledAt
			}
		}

		if dstURL != nil {
			page.Links = append(page.Links, *dstURL)
		}

		seen = true
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

	if !seen {
		return nil, fmt.Errorf("get page: %w", port.ErrPageNotFound)
	}

	return &page, nil
}

func (s *PageDS) PageExists(ctx context.Context, url string) (bool, error) {
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
	err := s.dbtx.QueryRow(ctx, sql, url).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("page exists: %w", err)
	}

	return exists, nil
}

func (s *PageDS) InsertPage(ctx context.Context, page *storage.PageDB) (int64, error) {
	sql := `
		INSERT INTO pages (url, title, description, content, crawled_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (url) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			content = EXCLUDED.content,
			crawled_at = EXCLUDED.crawled_at
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var pageID int64

	err := s.dbtx.QueryRow(ctx, sql, page.URL, page.Title, page.Description, page.Content, page.CrawledAt).Scan(&pageID)
	if err != nil {
		return -1, fmt.Errorf("insert page: %w", err)
	}

	return pageID, nil
}

func (s *PageDS) InsertLinks(ctx context.Context, links []*link) error {
	if len(links) == 0 {
		return nil
	}

	sql := `
		INSERT INTO links (src_page_id, dst_url)
		VALUES ($1, $2)
		ON CONFLICT (src_page_id, dst_url) DO NOTHING
	`

	batch := &pgx.Batch{}
	for _, link := range links {
		batch.Queue(sql, link.srcPageID, link.dstURL)
	}

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	br := s.dbtx.SendBatch(ctx, batch)
	defer func() {
		_ = br.Close()
	}()

	for range batch.Len() {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert links: %w", err)
		}
	}

	return nil
}

func (s *PageDS) withTX(tx pgx.Tx) *PageDS {
	return &PageDS{dbtx: tx}
}

func (s *PageDS) SavePage(ctx context.Context, page *storage.PageDB) error {
	wrap := func(err error) error {
		return fmt.Errorf("save page: %w", err)
	}

	tx, err := s.dbtx.Begin(ctx)
	if err != nil {
		return wrap(err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	txDS := s.withTX(tx)

	id, err := txDS.InsertPage(ctx, page)
	if err != nil {
		return wrap(err)
	}

	links := make([]*link, len(page.Links))
	for i, ref := range page.Links {
		links[i] = &link{srcPageID: id, dstURL: ref}
	}
	if err := txDS.InsertLinks(ctx, links); err != nil {
		return wrap(err)
	}

	if err = tx.Commit(ctx); err != nil {
		return wrap(err)
	}

	return nil
}
