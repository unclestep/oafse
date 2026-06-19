package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
	"oafse/internal/infrastructure/metrics"
)

type Index struct {
	cfg      *model.CrawlConfig
	pageRepo port.PageDBRepo
	urlRepo  port.URLRepo
	embedder port.Embedder
	proc     port.Processing
}

func NewIndex(cfg *model.CrawlConfig, pageRepo port.PageDBRepo, urlRepo port.URLRepo, embedder port.Embedder, proc port.Processing) *Index {
	return &Index{
		cfg:      cfg,
		pageRepo: pageRepo,
		urlRepo:  urlRepo,
		embedder: embedder,
		proc:     proc,
	}
}

func (uc *Index) Execute(ctx context.Context) {
	listenCtx, listenCancel := context.WithCancel(ctx)
	defer listenCancel()

	hasWork, errCh := uc.pageRepo.StartListeningPages(listenCtx)
	errCount := 0

	if err := uc.process(ctx); err != nil {
		errCount++
		log.Printf("[WARN] index cold start: %s", err)
	} else {
		errCount = 0
	}

	for {
		select {
		case <-hasWork:
			if err := uc.process(ctx); err != nil {
				errCount++
				log.Printf("[WARN] index process: %s", err)
			} else {
				errCount = 0
			}

			if errCount > 10 {
				log.Print("[ERR] index execute: too many errors")
				return
			}

			done, err := uc.checkDone(ctx)
			if err != nil {
				log.Printf("[ERR] index execute: %s", err)
				return
			}
			if done {
				log.Printf("[INFO] indexing is finished")
				return
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

			t := time.NewTimer(time.Until(retryAt))
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				return false, ctx.Err()
			}
		}

		crawlMeta, err := uc.urlRepo.GetCrawlMetadata(ctx)
		if err != nil {
			lastErr = err
			continue
		}

		pages, err := uc.pageRepo.GetUnvectorized(ctx)
		if err != nil {
			lastErr = err
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
	metrics.IndexerBacklog.Set(float64(len(pages)))
	if len(pages) == 0 {
		return nil
	}

	metrics.IndexerProgress.Set(float64(len(pages)))
	defer metrics.IndexerProgress.Set(0)

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
			continue
		}
		if len(p.Vector) > 0 {
			metrics.PagesEmbedded.Inc()
		}
	}

	return nil
}
