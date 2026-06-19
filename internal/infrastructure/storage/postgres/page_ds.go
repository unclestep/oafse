package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/pgvector/pgvector-go"
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

func (s *PageDS) GetPage(parent context.Context, url string) (*storage.PageDB, error) {
	query := `
		SELECT url, title, description, content, crawled_at, vector
		FROM pages
		WHERE url = $1
	`

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	defer observeQuery("get_page")()

	var page storage.PageDB
	var title, desc, content *string
	var vector *pgvector.Vector
	var crawledAt *time.Time
	err := s.dbtx.QueryRow(ctx, query, url).Scan(&page.URL, &title, &desc, &content, &crawledAt, &vector)
	if err != nil {
		return nil, wrapDBErr("get page", err)
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
	if vector != nil {
		page.Vector = (*vector).Slice()
	}

	return &page, nil
}

func (s *PageDS) PageExists(parent context.Context, url string) (bool, error) {
	query := `
		SELECT EXISTS(
			SELECT 1
			FROM pages
			WHERE url = $1
		)
	`

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	defer observeQuery("page_exists")()

	var exists bool
	err := s.dbtx.QueryRow(ctx, query, url).Scan(&exists)
	if err != nil {
		return false, wrapDBErr("page exists", err)
	}

	return exists, nil
}

func (s *PageDS) InsertPage(parent context.Context, page *storage.PageDB) (int64, error) {
	query := `
		INSERT INTO pages (url, title, description, content, crawled_at, vector, can_be_vectorized)
		VALUES ($1, $2, $3, $4, $5, l2_normalize($6::vector), $7)
		ON CONFLICT (url) DO UPDATE SET
			title = EXCLUDED.title,
			description = EXCLUDED.description,
			content = EXCLUDED.content,
			crawled_at = EXCLUDED.crawled_at,
			vector = l2_normalize(EXCLUDED.vector),
			can_be_vectorized = EXCLUDED.can_be_vectorized
		RETURNING id
	`

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	defer observeQuery("insert_page")()

	// page.Vector distinguishes three states: nil (not embedded yet, e.g. fresh crawl),
	// non-empty (a real vector), and non-nil-but-empty (the embedder tried and the text
	// was too large to embed) - that last case is the only one that should set
	// can_be_vectorized to false so GetUnvectorized stops retrying it forever.
	var vectorArg *pgvector.Vector
	canBeVectorized := true
	if len(page.Vector) > 0 {
		tmp := pgvector.NewVector(page.Vector)
		vectorArg = &tmp
	} else if page.Vector != nil {
		canBeVectorized = false
	}

	var pageID int64
	err := s.dbtx.QueryRow(ctx, query, page.URL, page.Title, page.Description, page.Content, page.CrawledAt, vectorArg, canBeVectorized).Scan(&pageID)
	if err != nil {
		return -1, wrapDBErr("insert page", err)
	}

	return pageID, nil
}

func (s *PageDS) InsertLink(parent context.Context, parentURL string, childPageID int64) error {
	query := `
		INSERT INTO links (src_page_id, dst_page_id)
		SELECT id, $2 FROM pages WHERE url = $1
		ON CONFLICT (src_page_id, dst_page_id) DO NOTHING
	`

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	defer observeQuery("insert_link")()

	_, err := s.dbtx.Exec(ctx, query, parentURL, childPageID)
	if err != nil {
		return wrapDBErr("insert link", err)
	}

	return nil
}

func (s *PageDS) GetUnvectorized(parent context.Context) ([]*storage.PageDB, error) {
	query := `
		SELECT url, title, description, content, crawled_at
		FROM pages
		WHERE vector IS NULL
			AND can_be_vectorized
			AND (title <> '' OR description <> '' OR content <> '')
	`

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	defer observeQuery("get_unvectorized")()

	rows, err := s.dbtx.Query(ctx, query)
	if err != nil {
		return nil, wrapDBErr("get unvectorized", err)
	}

	pages, err := pgx.CollectRows(rows, pgx.RowToAddrOfStructByNameLax[storage.PageDB])
	if err != nil {
		return nil, wrapDBErr("get unvectorized", err)
	}

	return pages, nil
}

func (s *PageDS) FindSimilar(parent context.Context, queryVector []float32, limit int) ([]*storage.PageDB, error) {
	// pgvector's <#> is the negative inner product; for L2-normalized vectors
	// this equals -cosine_similarity, ranging -1 (identical) to 1 (opposite).
	const minSimilarity = -0.3

	query := `
		SELECT url, title, description, content, crawled_at, vector
		FROM pages
		WHERE vector IS NOT NULL
			AND vector <#> l2_normalize($1::vector) < $2
		ORDER BY vector <#> l2_normalize($1::vector)
		LIMIT $3
	`

	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()
	defer observeQuery("find_similar")()

	rows, err := s.dbtx.Query(ctx, query, pgvector.NewVector(queryVector), minSimilarity, limit)
	if err != nil {
		return nil, wrapDBErr("find similar", err)
	}
	defer rows.Close()

	var pages []*storage.PageDB
	for rows.Next() {
		var page storage.PageDB
		var title, description, content *string
		var crawledAt *time.Time
		var vec pgvector.Vector
		if err := rows.Scan(&page.URL, &title, &description, &content, &crawledAt, &vec); err != nil {
			return nil, wrapDBErr("find similar", err)
		}

		page.Vector = vec.Slice()

		if title != nil {
			page.Title = *title
		}
		if description != nil {
			page.Description = *description
		}
		if content != nil {
			page.Content = *content
		}
		if crawledAt != nil {
			page.CrawledAt = *crawledAt
		}

		pages = append(pages, &page)
	}
	if err := rows.Err(); err != nil {
		return nil, wrapDBErr("find similar", err)
	}
	return pages, nil
}
