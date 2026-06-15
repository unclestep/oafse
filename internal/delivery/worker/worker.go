package worker

import (
	"context"
	"log"
	"strconv"
	"sync"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
)

type Worker struct {
	id    string
	parse port.ParseUseCase
}

func NewWorker(id string, parse port.ParseUseCase) *Worker {
	return &Worker{
		id:    id,
		parse: parse,
	}
}

type WorkerPool struct {
	wg      *sync.WaitGroup
	workers []*Worker
}

func NewWorkerPool(cfg *model.CrawlConfig, parse port.ParseUseCase) *WorkerPool {
	workers := make([]*Worker, cfg.WorkersCount)
	for i := range cfg.WorkersCount {
		workers[i] = NewWorker(strconv.Itoa(i), parse)
	}

	pool := &WorkerPool{
		wg:      &sync.WaitGroup{},
		workers: workers,
	}
	return pool
}

func (p *WorkerPool) Run(ctx context.Context) {
	for _, worker := range p.workers {
		p.wg.Go(func() {
			worker.run(ctx)
		})
	}
	p.wg.Wait()
	log.Printf("[INFO] parsing is finished")
}

func (w *Worker) run(parent context.Context) {
	for {
		ctx, cancel := context.WithTimeout(parent, 15*time.Second)

		ch := make(chan *port.ParseCmd, 1)

		go func() {
			cmd, _ := w.parse.Execute(ctx, w.id)
			ch <- cmd
		}()

		select {
		case cmd := <-ch:
			cancel()

			if cmd == nil {
				break
			}
			switch cmd.Directive {
			case port.DirectiveSleep:
				select {
				case <-time.After(cmd.SleepFor):
				case <-parent.Done():
					return
				}
			case port.DirectiveStop:
				return
			}
		case <-ctx.Done():
			if parent.Err() != nil {
				cancel()
				return
			}
		}
	}
}
