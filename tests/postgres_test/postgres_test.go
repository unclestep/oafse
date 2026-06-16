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

func (s *PostgresSuite) linkExists(srcPageID, dstPageID int64) bool {
	var exists bool
	err := s.tx.QueryRow(
		context.Background(),
		"SELECT EXISTS(SELECT 1 FROM links WHERE src_page_id = $1 AND dst_page_id = $2)",
		srcPageID, dstPageID,
	).Scan(&exists)
	s.Require().NoError(err)
	return exists
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

	parentID, err := ds.InsertPage(ctx, &storage.PageDB{URL: "PARENT", Title: "Parent"})
	s.Require().NoError(err)

	child1ID, err := ds.InsertPage(ctx, &storage.PageDB{URL: "CHILD1", Title: "Child1"})
	s.Require().NoError(err)

	child2ID, err := ds.InsertPage(ctx, &storage.PageDB{URL: "CHILD2", Title: "Child2"})
	s.Require().NoError(err)

	s.Run("Insert link to existing parent", func() {
		before := s.countRows("links")

		err := ds.InsertLink(ctx, "PARENT", child1ID)
		s.NoError(err)

		s.Equal(before+1, s.countRows("links"))
		s.True(s.linkExists(parentID, child1ID))
	})

	s.Run("Insert another link from the same parent", func() {
		before := s.countRows("links")

		err := ds.InsertLink(ctx, "PARENT", child2ID)
		s.NoError(err)

		s.Equal(before+1, s.countRows("links"))
		s.True(s.linkExists(parentID, child2ID))
	})

	s.Run("Insert duplicate link is idempotent", func() {
		before := s.countRows("links")

		err := ds.InsertLink(ctx, "PARENT", child1ID)
		s.NoError(err)

		s.Equal(before, s.countRows("links"), "ON CONFLICT DO NOTHING must not duplicate links")
	})

	s.Run("Insert link with unknown parent URL is a no-op", func() {
		before := s.countRows("links")

		err := ds.InsertLink(ctx, "UNKNOWN-PARENT", child1ID)
		s.NoError(err)

		s.Equal(before, s.countRows("links"), "no matching parent row means nothing should be inserted")
	})
}

func TestPostgresSuite(t *testing.T) {
	suite.Run(t, new(PostgresSuite))
}
