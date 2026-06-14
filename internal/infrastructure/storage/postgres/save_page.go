package postgres

import (
	"context"
	"fmt"
	"time"

	storage "oafse/internal/infrastructure/storage/model"

	"github.com/jackc/pgx/v5"
)

func InsertPage(ctx context.Context, db DBTX, page *storage.PageDB) (int64, error) {
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

	err := db.QueryRow(ctx, sql, page.URL, page.Title, page.Description, page.Content, page.CrawledAt).Scan(&pageID)
	if err != nil {
		return -1, fmt.Errorf("insert page: %w", err)
	}

	return pageID, nil
}

func InsertLinks(ctx context.Context, db DBTX, links []*link) error {
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

	br := db.SendBatch(ctx, batch)
	defer br.Close()

	for range batch.Len() {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("insert links: %w", err)
		}
	}

	return nil
}

func SavePage(ctx context.Context, txb TxBeginner, page *storage.PageDB) error {
	wrap := func(err error) error {
		return fmt.Errorf("save page: %w", err)
	}

	tx, err := txb.Begin(ctx)
	if err != nil {
		return wrap(err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	id, err := InsertPage(ctx, tx, page)
	if err != nil {
		return wrap(err)
	}

	links := make([]*link, len(page.Links))
	for i, ref := range page.Links {
		links[i] = &link{srcPageID: id, dstURL: ref}
	}

	if err := InsertLinks(ctx, tx, links); err != nil {
		return wrap(err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return wrap(err)
	}

	return nil
}
