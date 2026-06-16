package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

type Index struct {
	cfg      *model.CrawlConfig
	pageRepo port.PageDBRepo
	urlRepo  port.URLRepo
	embedder port.Embedder
	proc     port.Processing
}

func NewIndex(cfg *model.CrawlConfig, pageRepo port.PageDBRepo, urlRepo port.URLRepo, embedder port.Embedder) *Index {
	return &Index{
		cfg:      cfg,
		pageRepo: pageRepo,
		urlRepo:  urlRepo,
		embedder: embedder,
	}
}

func (uc *Index) Execute(ctx context.Context) {
	hasWork := make(chan bool, 1)
	errCh := make(chan error, 1)

	go func() {
		for {
			done, err := uc.checkDone(ctx)
			if err != nil {
				if ctx.Err() != nil {
					hasWork <- false
					return
				}
				errCh <- err
				return
			}
			if done {
				hasWork <- false
				return
			}

			if err := uc.pageRepo.WaitForNotification(ctx); err != nil {
				if ctx.Err() != nil {
					hasWork <- false
				}
				errCh <- fmt.Errorf("wait for notification: %w", err)
				return
			}

			select {
			case hasWork <- true:
			default:
			}
		}
	}()

	if err := uc.process(ctx); err != nil {
		log.Printf("[WARN] index cold start: %s", err)
	}

	for {
		select {
		case has := <-hasWork:
			if !has {
				return
			}

			if err := uc.process(ctx); err != nil {
				log.Printf("[WARN] index process: %s", err)
			}
		case err := <-errCh:
			log.Printf("[ERR] index execute: %s", err)
			return
		case <-ctx.Done():
			return
		}
	}
}

func (uc *Index) checkDone(ctx context.Context) (bool, error) {
	var lastErr error

	for i := 0; i < uc.cfg.TryLim; i++ {
		if i > 0 {
			_, retryAt := uc.proc.CalcRetryTime(i, uc.cfg)
			t := time.NewTicker(time.Until(retryAt))
			select {
			case <-t.C:
			case <-ctx.Done():
				return false, ctx.Err()
			}
		}

		crawlMeta, err := uc.urlRepo.GetCrawlMetadata(ctx)
		if err != nil {
			continue
		}

		pages, err := uc.pageRepo.GetUnvectorized(ctx)
		if err != nil {
			continue
		}
		return crawlMeta.UnprocessedCount == 0 && len(pages) == 0, nil
	}

	return false, fmt.Errorf("after %d retries there is still a problem with repos, last error: %w", uc.cfg.TryLim, lastErr)
}

func (uc *Index) process(parent context.Context) error {
	pages, err := uc.pageRepo.GetUnvectorized(parent)
	if err != nil {
		return fmt.Errorf("get unvectorized: %w", err)
	}
	if len(pages) == 0 {
		return nil
	}

	timeout := time.Duration(len(pages)) * 100 * time.Millisecond
	ctx, cancel := context.WithTimeout(parent, timeout)
	defer cancel()

	texts := make([]string, len(pages))
	for i, p := range pages {
		texts[i] = p.FormatPageText()
	}

	vectors, err := uc.embedder.EmbedBatch(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed batch: %w", err)
	}

	for i, p := range pages {
		p.Vector = vectors[i]
		if _, err := uc.pageRepo.SavePage(ctx, p); err != nil {
			log.Printf("[WARN] save vector page %s: %s", p.URL, err)
		}
	}

	return nil
}
