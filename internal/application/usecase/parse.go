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
	pageRepo  port.PageDBRepo
	urlRepo   port.URLRepo
	fetcher   port.Fetcher
	proc      port.Processing
	extractor port.Extractor
}

func NewParse(cfg *model.CrawlConfig, pageRepo port.PageDBRepo, urlRepo port.URLRepo, proc port.Processing, fetcher port.Fetcher, extractor port.Extractor) *Parse {
	return &Parse{
		cfg:       cfg,
		pageRepo:  pageRepo,
		urlRepo:   urlRepo,
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

		if err := uc.urlRepo.MarkProcessed(cleanupCtx, url.String(), model.GiveUpCrawlStatus); err != nil {
			return nil, wrap(err)
		}
		return nil, nil
	}

	url, err := uc.urlRepo.TakeOn(ctx, workerID)
	if errors.Is(err, port.ErrQueueEmpty) {
		md, err := uc.urlRepo.GetCrawlMetadata(ctx)
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
		retry, retryAt := uc.proc.CalcRetryTime(url.Try, uc.cfg)
		if !retry {
			return giveUp(url, fmt.Errorf("retry limit exceeded: cur %d, threshold %d", url.Try, uc.cfg.TryLim))
		}

		if err = uc.urlRepo.RetryURL(ctx, url.String(), retryAt); err != nil {
			return giveUp(url, err)
		}
		return nil, nil
	}

	page, links, err := uc.extractor.Extract(fd)
	if err != nil {
		return giveUp(url, err)
	}

	pageID, err := uc.pageRepo.SavePage(ctx, page)
	if err != nil {
		log.Printf("[WARN] lost urls: %v", links)
		return giveUp(url, err)
	}

	if url.Parent != "" {
		if err := uc.pageRepo.SaveLink(ctx, url.Parent, pageID); err != nil {
			log.Printf("[WARN] lost urls: %v", links)
			return giveUp(url, err)
		}
	}

	if len(links) != 0 {
		if _, err := uc.urlRepo.PushURLs(ctx, links); err != nil {
			return giveUp(url, err)
		}
	}

	err = uc.urlRepo.MarkProcessed(ctx, url.String(), model.DoneCrawlStatus)
	if err != nil {
		return giveUp(url, err)
	}

	log.Printf("[INFO] page %s is successfully parsed", page.URL)
	return nil, nil
}
