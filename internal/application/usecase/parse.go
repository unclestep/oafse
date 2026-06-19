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
	"oafse/internal/infrastructure/metrics"
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

func (uc *Parse) markTerminal(url *model.URL, status model.CrawlStatus, label string, err error) (*port.ParseCmd, error) {
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("[WARN] parse use case: marking %s as %s because of: %s", url.String(), status.String(), err)

	metrics.FetchAttempts.WithLabelValues(label).Inc()

	if err := uc.urlRepo.MarkProcessed(cleanupCtx, url.String(), status); err != nil {
		return nil, fmt.Errorf("parse use case: %w", err)
	}
	return nil, nil
}

func (uc *Parse) giveUp(url *model.URL, err error) (*port.ParseCmd, error) {
	return uc.markTerminal(url, model.GiveUpCrawlStatus, metrics.StatusGiveUp, err)
}

func (uc *Parse) leaveForRecovery(url *model.URL, err error) (*port.ParseCmd, error) {
	log.Printf("[WARN] parse use case: transient error for %s, leaving for reaper recovery: %s", url.String(), err)
	return nil, nil
}

func (uc *Parse) Execute(ctx context.Context, workerID string) (*port.ParseCmd, error) {
	wrap := func(err error) error {
		return fmt.Errorf("parse use case: %w", err)
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
		return uc.giveUp(url, err)
	}
	switch fd.Status {
	case port.FetchOK, port.FetchRetry:
		// handled below
	case port.FetchManual:
		return uc.markTerminal(url, model.ManualCrawlStatus, metrics.StatusManual, fmt.Errorf("requires manual review: %v", fd.Status.String()))
	default:
		return uc.markTerminal(url, model.GiveUpCrawlStatus, metrics.StatusImpossible, fmt.Errorf("unrecoverable url status: %v", fd.Status.String()))
	}

	if fd.Status == port.FetchRetry {
		retry, retryAt := uc.proc.CalcRetryTime(url.Try, uc.cfg)
		if !retry {
			return uc.giveUp(url, fmt.Errorf("retry limit exceeded: cur %d, threshold %d", url.Try, uc.cfg.TryLim))
		}

		if err = uc.urlRepo.RetryURL(ctx, url.String(), retryAt); err != nil {
			return uc.giveUp(url, err)
		}
		metrics.FetchAttempts.WithLabelValues(metrics.StatusRetry).Inc()
		return nil, nil
	}

	page, links, err := uc.extractor.Extract(fd)
	if err != nil {
		return uc.giveUp(url, err)
	}

	pageID, err := uc.pageRepo.SavePage(ctx, page)
	if err != nil {
		if errors.Is(err, port.ErrTransient) {
			return uc.leaveForRecovery(url, err)
		}
		log.Printf("[WARN] lost urls: %v", links)
		return uc.giveUp(url, err)
	}

	if url.Parent != "" {
		if err := uc.pageRepo.SaveLink(ctx, url.Parent, pageID); err != nil {
			if errors.Is(err, port.ErrTransient) {
				return uc.leaveForRecovery(url, err)
			}
			log.Printf("[WARN] lost urls: %v", links)
			return uc.giveUp(url, err)
		}
	}

	if len(links) != 0 {
		if _, err := uc.urlRepo.PushURLs(ctx, links); err != nil {
			return uc.giveUp(url, err)
		}
	}

	err = uc.urlRepo.MarkProcessed(ctx, url.String(), model.DoneCrawlStatus)
	if err != nil {
		return uc.giveUp(url, err)
	}

	metrics.FetchAttempts.WithLabelValues(metrics.StatusOK).Inc()
	log.Printf("[INFO] page %s is successfully parsed", page.URL)
	return nil, nil
}
