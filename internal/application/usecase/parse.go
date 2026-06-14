package usecase

import (
	"context"
	"errors"
	"fmt"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

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

func (p *Parse) Execute(ctx context.Context) (*port.ParseCmd, error) {
	wrap := func(err error) error {
		return fmt.Errorf("parse: %w", err)
	}

	url, err := p.repo.TakeOn(ctx)
	if errors.Is(err, port.ErrQueueEmpty) {
		done, err := p.repo.IsCrawlComplete(ctx)
		if err != nil {
			return nil, wrap(err)
		}
		if done {
			return &port.ParseCmd{Directive: port.DirectiveStop, SleepFor: 0}, nil
		}
		return &port.ParseCmd{
			Directive: port.DirectiveSleep,
			SleepFor:  100,
		}, nil
	}
	if err != nil {
		return nil, wrap(err)
	}

	fd, err := p.fetcher.Fetch(ctx, url)
	if err != nil {
		return nil, wrap(err)
	}

	retry := p.proc.DecideRetry(url, p.cfg)
	if retry || fd.Status == port.FetchRetry {
		p.repo.Retry(ctx, url)
		return &port.ParseCmd{Directive: port.DirectiveContinue, SleepFor: 0}, nil
	}

	page, err := p.extractor.Extract(fd)
	if err != nil {
		return nil, wrap(err)
	}

	return url, nil
}
