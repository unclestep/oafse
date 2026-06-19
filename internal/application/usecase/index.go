package usecase

import (
	"context"
	"fmt"
	"log"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/infrastructure/metrics"
)

type Index struct {
	pageRepo port.PageDBRepo
	urlRepo  port.URLRepo
	embedder port.Embedder
}

func NewIndex(pageRepo port.PageDBRepo, urlRepo port.URLRepo, embedder port.Embedder) *Index {
	return &Index{
		pageRepo: pageRepo,
		urlRepo:  urlRepo,
		embedder: embedder,
	}
}

func (uc *Index) Execute(ctx context.Context) {
	listenCtx, listenCancel := context.WithCancel(ctx)
	defer listenCancel()

	hasWork, errCh := uc.pageRepo.StartListeningPages(listenCtx)

	queueInsert, err := uc.urlRepo.Subscribe(listenCtx)
	if err != nil {
		log.Printf("[ERR] index execute: subscribe: %s", err)
		return
	}

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
		case <-queueInsert:
			if err := uc.process(ctx); err != nil {
				errCount++
				log.Printf("[WARN] index process: %s", err)
			} else {
				errCount = 0
			}
		case err := <-errCh:
			errCount++
			log.Printf("[WARN] index execute: listen: %s", err)
		case <-ctx.Done():
			return
		}

		if errCount > 10 {
			log.Print("[ERR] index execute: too many errors")
			return
		}
	}
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
