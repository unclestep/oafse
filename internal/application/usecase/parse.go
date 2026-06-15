package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

type Parse struct {
	cfg       *model.CrawlConfig
	repo      port.CrawlRepo
	fetcher   port.Fetcher
	proc      port.Processing
	extractor port.Extractor
}

func NewParse(cfg *model.CrawlConfig, repo port.CrawlRepo, proc port.Processing, fetcher port.Fetcher, extractor port.Extractor) *Parse {
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
		return fmt.Errorf("parse use case: %w", err)
	}
	giveUp := func(url *model.URL, err error) (*port.ParseCmd, error) {
		log.Printf("[ERROR] parse use case: give up url because of: %s", err)
		if err := uc.repo.GiveUpURL(ctx, url.String()); err != nil {
			return nil, wrap(err)
		}
		return nil, nil
	}

	url, err := uc.repo.TakeOn(ctx, workerID)
	if errors.Is(err, port.ErrQueueEmpty) {
		md, err := uc.repo.GetCrawlMetadata(ctx)
		if err != nil {
			return nil, wrap(err)
		}
		if md.UnprocessedCount > 0 {
			var retryAt = time.Now().Add(rand.N(100 * time.Millisecond))
			if !md.EarliestRetry.IsZero() {
				retryAt = md.EarliestRetry
			}
			return &port.ParseCmd{Directive: port.DirectiveSleep, SleepFor: time.Until(retryAt)}, nil
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
		return giveUp(url, err)
	}
	if fd.Status == port.FetchImpossible || fd.Status == port.FetchManual {
		return giveUp(url, fmt.Errorf("unrecoverable url status: %v", fd.Status))
	}

	retry, retryAt := uc.proc.DecideRetry(url, uc.cfg)
	if retry && fd.Status == port.FetchRetry {
		if err = uc.repo.RetryURL(ctx, url.String(), retryAt); err != nil {
			return giveUp(url, err)
		}
		return nil, nil
	} else if !retry && fd.Status == port.FetchRetry {
		return giveUp(url, fmt.Errorf("retry limit exceeded: cur %d, threshold %d", url.Try, uc.cfg.TryLim))
	}

	page, err := uc.extractor.Extract(fd)
	if err != nil {
		return giveUp(url, err)
	}

	if err := uc.repo.Done(ctx, page); err != nil {
		return giveUp(url, err)
	}

	log.Printf("[INFO] page %s is successfully parsed", page.URL)

	return nil, nil
}
