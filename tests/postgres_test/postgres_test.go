package postgres_test

import (
	"context"
	"fmt"
	"testing"

	pg "oafse/internal/infrastructure/storage/postgres"
	storage "oafse/internal/infrastructure/storage/model"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type PostgresSuite struct {
	suite.Suite
	container *postgres.PostgresContainer
	pool      *pgxpool.Pool
	tx        pgx.Tx
}

func (s *PostgresSuite) SetupSuite() {
	ctx := context.Background()

	container, err := postgres.Run(ctx,
		"postgres:18-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("dbuser"),
		postgres.WithPassword("dbpassword"),
		postgres.BasicWaitStrategies(),
	)

	s.Require().NoError(err)
	s.container = container

	databaseURL, err := container.ConnectionString(ctx, "sslmode=disable")
	s.Require().NoError(err)

	err = pg.Migrate(databaseURL)
	s.Require().NoError(err)

	pool, err := pg.NewPool(databaseURL)
	s.Require().NoError(err)
	s.pool = pool
}

func (s *PostgresSuite) TearDownSuite() {
	s.pool.Close()
	err := testcontainers.TerminateContainer(s.container)
	s.Require().NoError(err)
}

func (s *PostgresSuite) SetupTest() {
	tx, err := s.pool.Begin(context.Background())
	s.Require().NoError(err)
	s.tx = tx
}

func (s *PostgresSuite) TearDownTest() {
	s.tx.Rollback(context.Background()) //nolint:errcheck
}

func (s *PostgresSuite) ds() *pg.PageDS {
	return pg.NewPageDS(s.tx)
}

func (s *PostgresSuite) countRows(table string) int {
	var n int
	err := s.tx.QueryRow(context.Background(), "SELECT count(*) FROM "+table).Scan(&n)
	s.Require().NoError(err)
	return n
}

func (s *PostgresSuite) TestInsertPage() {
	ds := s.ds()
	ctx := context.Background()

	validPages := []*storage.PageDB{
		{URL: "URL1", Title: "Title1"},
		{URL: "URL2", Title: "Title2"},
		{URL: "URL3", Title: "Title3"},
	}

	for _, page := range validPages {
		s.Run(fmt.Sprintf("Insert valid Page: %s", page.URL), func() {
			id, err := ds.InsertPage(ctx, page)
			s.NoError(err)
			s.Greater(id, int64(0))
		})
	}

	s.Equal(3, s.countRows("pages"))

	existedPages := []*storage.PageDB{
		{URL: "URL1", Title: "NewTitle1"},
		{URL: "URL2", Title: "NewTitle2"},
	}

	for _, page := range existedPages {
		s.Run(fmt.Sprintf("Upsert existing Page: %s", page.URL), func() {
			before := s.countRows("pages")

			_, err := ds.InsertPage(ctx, page)
			s.NoError(err)

			exists, err := ds.PageExists(ctx, page.URL)
			s.NoError(err)
			s.True(exists)

			updated, err := ds.GetPage(ctx, page.URL)
			s.NoError(err)
			s.Equal(page.Title, updated.Title)
			s.Equal(page.URL, updated.URL)

			s.Equal(before, s.countRows("pages"), "row count must not increase on conflict")
		})
	}
}

func (s *PostgresSuite) TestInsertLink() {
	ds := s.ds()
	ctx := context.Background()

	pages := []*storage.PageDB{
		{URL: "URL1", Title: "Title1", Links: []string{"URL2", "URL3"}},
		{URL: "URL2", Title: "Title2", Links: []string{"URL1"}},
		{URL: "URL3", Title: "Title3", Links: []string{"URL2"}},
	}

	for _, page := range pages {
		s.Run(fmt.Sprintf("Insert links for %s", page.URL), func() {
			before := s.countRows("links")

			err := ds.SavePage(ctx, page)
			s.NoError(err)

			s.Equal(before+len(page.Links), s.countRows("links"))

			saved, err := ds.GetPage(ctx, page.URL)
			s.NoError(err)
			s.ElementsMatch(page.Links, saved.Links)
		})
	}

	totalLinks := 2 + 1 + 1 // URL1→2, URL1→3, URL2→1, URL3→2
	s.Equal(totalLinks, s.countRows("links"))

	for _, page := range pages {
		s.Run(fmt.Sprintf("Insert duplicate links for %s is idempotent", page.URL), func() {
			before := s.countRows("links")

			err := ds.SavePage(ctx, page)
			s.NoError(err)

			s.Equal(before, s.countRows("links"), "ON CONFLICT DO NOTHING must not duplicate links")
		})
	}
}

func (s *PostgresSuite) TestSavePageCrawl() {
	ds := s.ds()
	ctx := context.Background()

	tests := []struct {
		page  *storage.PageDB
	}{
		{&storage.PageDB{URL: "URL4", Title: "Title4", Links: []string{"URL1", "URL2", "URL3"}}},
		{&storage.PageDB{URL: "URL5", Title: "Title5", Links: []string{"URL1", "URL2", "URL3"}}},
	}

	for _, tc := range tests {
		s.Run(fmt.Sprintf("Page Crawl: %s", tc.page.URL), func() {
			before := s.countRows("links")

			err := ds.SavePage(ctx, tc.page)
			s.NoError(err)

			exists, err := ds.PageExists(ctx, tc.page.URL)
			s.NoError(err)
			s.True(exists)

			saved, err := ds.GetPage(ctx, tc.page.URL)
			s.NoError(err)
			s.ElementsMatch(tc.page.Links, saved.Links)

			s.Equal(before+len(tc.page.Links), s.countRows("links"))
		})
	}
}

func TestPostgresSuite(t *testing.T) {
	suite.Run(t, new(PostgresSuite))
}
