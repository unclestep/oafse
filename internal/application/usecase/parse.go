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
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log.Printf("[WARN] parse use case: give up %s because of: %s", url.String(), err)

		if err := uc.repo.GiveUpURL(cleanupCtx, url.String()); err != nil {
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
			retryAt := time.Now().Add(rand.N(100 * time.Millisecond))
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
	if fd.Status != port.FetchOK && fd.Status != port.FetchRetry {
		return giveUp(url, fmt.Errorf("unrecoverable url status: %v", fd.Status.String()))
	}

	if fd.Status == port.FetchRetry {
		retry, retryAt := uc.proc.DecideRetry(url, uc.cfg)
		if !retry {
			return giveUp(url, fmt.Errorf("retry limit exceeded: cur %d, threshold %d", url.Try, uc.cfg.TryLim))
		}

		if err = uc.repo.RetryURL(ctx, url.String(), retryAt); err != nil {
			return giveUp(url, err)
		}
		return nil, nil
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
