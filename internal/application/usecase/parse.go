package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"oafse/internal/application/port"
)

type Parse struct {
	cfg       *port.CrawlConfig
	repo      port.CrawlRepo
	fetcher   port.Fetcher
	proc      port.Processing
	extractor port.Extractor
}

func NewParse(cfg *port.CrawlConfig, repo port.CrawlRepo, proc port.Processing, fetcher port.Fetcher, extractor port.Extractor) *Parse {
	return &Parse{
		cfg:       cfg,
		repo:      repo,
		fetcher:   fetcher,
		proc:      proc,
		extractor: extractor,
	}
}

func (uc *Parse) Execute(ctx context.Context, workerID string) (*port.ParseCmd, error) {
	wrap := func(err error) error {
		return fmt.Errorf("parse: %w", err)
	}

	url, err := uc.repo.TakeOn(ctx, workerID)
	if errors.Is(err, port.ErrQueueEmpty) {
		md, err := uc.repo.GetCrawlMetadata(ctx)
		if err != nil {
			return nil, wrap(err)
		}
		if md.UnprocessedCount > 0 {
			return &port.ParseCmd{Directive: port.DirectiveSleep, SleepFor: md.EarliestRetry.Sub(time.Now())}, nil
		}
		return &port.ParseCmd{
			Directive: port.DirectiveStop,
			SleepFor:  0,
		}, nil
	}
	if err != nil {
		return nil, wrap(err)
	}

	fd, err := uc.fetcher.Fetch(ctx, url)
	if err != nil {
		if err := uc.repo.GiveUpURL(ctx, url.String()); err != nil {
			return nil, wrap(err)
		}
	}

	retry := uc.proc.DecideRetry(url, uc.cfg)
	if retry && fd.Status == port.FetchRetry {
		uc.repo.RetryURL(ctx, url.String(), url.RetryAt)
		return nil, nil
	} else if !retry && fd.Status == port.FetchRetry {
		if err := uc.repo.GiveUpURL(ctx, url.String()); err != nil {
			return nil, wrap(err)
		}
	}

	page, err := uc.extractor.Extract(fd)
	if err != nil {
		if err := uc.repo.GiveUpURL(ctx, url.String()); err != nil {
			return nil, wrap(err)
		}
	}

	if err := uc.repo.Done(ctx, page); err != nil {
		if err := uc.repo.GiveUpURL(ctx, url.String()); err != nil {
			return nil, wrap(err)
		}
	}

	return nil, nil
}
