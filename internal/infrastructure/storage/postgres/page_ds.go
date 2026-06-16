package postgres

import (
	"context"
	"fmt"
	"time"

	storage "oafse/internal/infrastructure/storage/model"
)

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
		SELECT url, title, description, content, crawled_at
		FROM pages
		WHERE url = $1
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var page storage.PageDB
	var title, desc, content *string
	var crawledAt *time.Time
	err := s.dbtx.QueryRow(ctx, sql, url).Scan(&page.URL, &title, &desc, &content, &crawledAt)
	if err != nil {
		return nil, fmt.Errorf("get page: %w", err)
	}

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

func (s *PageDS) InsertLink(ctx context.Context, parentURL string, childPageID int64) error {
	sql := `
		INSERT INTO links (src_page_id, dst_page_id)
		SELECT id, $2 FROM pages WHERE url = $1
		ON CONFLICT (src_page_id, dst_page_id) DO NOTHING
	`

	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	_, err := s.dbtx.Exec(ctx, sql, parentURL, childPageID)
	if err != nil {
		return fmt.Errorf("insert link: %w", err)
	}

	return nil
}
