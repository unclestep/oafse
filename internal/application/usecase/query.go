package usecase

import (
	"context"
	"fmt"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

type Query struct {
	pageRepo port.PageDBRepo
	embedder port.Embedder
}

func NewQuery(pageRepo port.PageDBRepo, embedder port.Embedder) *Query {
	return &Query{
		pageRepo: pageRepo,
		embedder: embedder,
	}
}

func (uc *Query) Execute(parent context.Context, query string, limit int) ([]*model.Page, error) {
	ctx, cancel := context.WithTimeout(parent, 15*time.Second)
	defer cancel()

	vector, err := uc.embedder.Embed(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	pages, err := uc.pageRepo.FindSimilar(ctx, vector, limit)
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}

	return pages, err
}
