package postgres

import (
	"context"
	"fmt"
	"time"

	"oafse/internal/application/port"
	storage "oafse/internal/infrastructure/storage/model"
)

func GetPage(ctx context.Context, db DBTX, pageURL string) (*storage.PageDB, error) {
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
	rows, err := db.Query(ctx, sql, pageURL)
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
