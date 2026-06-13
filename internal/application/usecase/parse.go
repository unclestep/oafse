package usecase

import (
	"context"
	"fmt"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

type ParseUseCase interface {
	Execute(ctx context.Context) error
}

type Parse struct {
	cfg       *port.ParseConfig
	repo      port.PageRepo
	fetcher   port.Fetcher
	proc      port.Processing
	extractor port.Extractor
}

func NewParse(cfg *port.ParseConfig, repo port.PageRepo, proc port.Processing, fetcher port.Fetcher, extractor port.Extractor) *Parse {
	return &Parse{
		cfg:       cfg,
		repo:      repo,
		fetcher:   fetcher,
		proc:      proc,
		extractor: extractor,
	}
}

func (p *Parse) Execute(ctx context.Context) (*model.URL, error) {
	wrap := func(err error) error {
		return fmt.Errorf("parse: %w", err)
	}

	url, err := p.repo.TakeOn(ctx)
	if err != nil {
		return nil, wrap(err)
	}

	res, err := p.fetcher.Fetch(ctx, url)
	if err != nil {
		return nil, wrap(err)
	}

	retry := p.proc.DecideRetry(url, p.cfg)
	if retry || res.Status == port.FetchRetry {
		p.repo.Retry(ctx, url)
		return url, nil
	}

	return url, nil
}
