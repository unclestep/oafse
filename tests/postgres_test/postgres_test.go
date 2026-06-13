package postgres_test

import (
	"context"
	"fmt"
	"testing"

	pg "oafse/internal/infrastructure/storage/postgres"

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

func (s *PostgresSuite) TestInsertPage() {
	validPages := []*pg.Page{
		{URL: "URL1", Title: "Title1", Status: 200},
		{URL: "URL2", Title: "Title2", Status: 200},
		{URL: "URL3", Title: "Title3", Status: 200},
	}

	for _, page := range validPages {
		subtestName := fmt.Sprintf("Insert valid Page: %s", page.URL)
		s.Run(subtestName, func() {
			id, err := pg.InsertPage(context.Background(), s.tx, page)
			s.NoError(err)
			s.NotEqual(-1, id)
		})
	}

	existedPages := []*pg.Page{
		{URL: "URL1", Title: "NewTitle1", Status: 200},
		{URL: "URL2", Title: "NewTitle2", Status: 200},
	}

	for _, page := range existedPages {
		subtestName := fmt.Sprintf("Insert existed Page: %s", page.URL)
		s.Run(subtestName, func() {
			var before, after int
			ctx := context.Background()

			err := s.tx.QueryRow(ctx, "SELECT count(*) FROM pages").Scan(&before)
			s.NoError(err)

			_, err = pg.InsertPage(ctx, s.tx, page)
			s.NoError(err)

			exists, err := pg.PageExists(ctx, s.tx, page.URL)
			s.NoError(err)
			s.True(exists)

			updatedPage, err := pg.GetPage(context.Background(), s.tx, page.URL)
			s.NoError(err)
			s.Equal(updatedPage.Title, page.Title)
			s.Equal(updatedPage.URL, page.URL)

			err = s.tx.QueryRow(context.Background(), "SELECT count(*) FROM pages").Scan(&after)
			s.NoError(err)

			s.Equal(before, after)
		})
	}
}

func (s *PostgresSuite) insertTestPages() []*pg.Page {
	pages := []*pg.Page{
		{URL: "URL1", Title: "Title1", Status: 200},
		{URL: "URL2", Title: "Title2", Status: 200},
		{URL: "URL3", Title: "Title3", Status: 200},
	}
	for _, page := range pages {
		id, err := pg.InsertPage(context.Background(), s.tx, page)
		s.Require().NoError(err)
		s.NotEqual(-1, id)
		page.ID = id
	}
	return pages
}

func (s *PostgresSuite) TestInsertLink() {
	pages := s.insertTestPages()

	validLinks := []*pg.Link{
		{SrcPageID: pages[0].ID, DstURL: "URL2"},
		{SrcPageID: pages[0].ID, DstURL: "URL3"},
		{SrcPageID: pages[1].ID, DstURL: "URL1"},
		{SrcPageID: pages[2].ID, DstURL: "URL2"},
	}

	for _, link := range validLinks {
		subtestName := fmt.Sprintf("Insert valid link %d-%s", link.SrcPageID, link.DstURL)
		s.Run(subtestName, func() {
			var before, after int
			ctx := context.Background()

			err := s.tx.QueryRow(ctx, "SELECT count(*) FROM links").Scan(&before)
			s.NoError(err)

			err = pg.InsertLink(context.Background(), s.tx, link)
			s.NoError(err)

			err = s.tx.QueryRow(ctx, "SELECT count(*) FROM links").Scan(&after)
			s.NoError(err)

			s.Equal(before+1, after)
		})
	}

	existedLinks := validLinks

	for _, link := range existedLinks {
		subtestName := fmt.Sprintf("Insert existed link %d-%s", link.SrcPageID, link.DstURL)
		s.Run(subtestName, func() {
			var before, after int
			ctx := context.Background()

			err := s.tx.QueryRow(ctx, "SELECT count(*) FROM links").Scan(&before)
			s.NoError(err)

			err = pg.InsertLink(ctx, s.tx, link)
			s.NoError(err)

			err = s.tx.QueryRow(ctx, "SELECT count(*) FROM links").Scan(&after)
			s.NoError(err)

			s.Equal(after, before)
		})
	}
}

func (s *PostgresSuite) TestSavePageCrawl() {
	pages := s.insertTestPages()

	tests := []struct {
		Page  *pg.Page
		Links []*pg.Link
	}{
		{
			&pg.Page{URL: "URL4", Title: "Title4", Status: 200},
			[]*pg.Link{
				{SrcPageID: pages[0].ID, DstURL: "URL4"},
				{SrcPageID: pages[1].ID, DstURL: "URL4"},
				{SrcPageID: pages[2].ID, DstURL: "URL4"},
			},
		},
		{
			&pg.Page{URL: "URL5", Title: "Title5", Status: 200},
			[]*pg.Link{
				{SrcPageID: pages[0].ID, DstURL: "URL5"},
				{SrcPageID: pages[1].ID, DstURL: "URL5"},
				{SrcPageID: pages[2].ID, DstURL: "URL5"},
			},
		},
	}

	for _, test := range tests {
		subtestName := fmt.Sprintf("Page Crawl: %s", test.Page.URL)
		s.Run(subtestName, func() {
			var before, after int
			ctx := context.Background()

			err := s.tx.QueryRow(ctx, "SELECT count(*) FROM links").Scan(&before)
			s.NoError(err)

			err = pg.SavePageCrawl(ctx, s.tx, test.Page, test.Links)
			s.NoError(err)

			exists, err := pg.PageExists(ctx, s.tx, test.Page.URL)
			s.NoError(err)
			s.True(exists)

			err = s.tx.QueryRow(ctx, "SELECT count(*) FROM links").Scan(&after)
			s.NoError(err)

			s.Equal(before+len(test.Links), after)
		})
	}
}

func TestPostgresSuite(t *testing.T) {
	suite.Run(t, new(PostgresSuite))
}
