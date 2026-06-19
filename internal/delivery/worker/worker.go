package worker

import (
	"context"
	"log"
	"runtime/debug"
	"strconv"
	"sync"
	"time"

	"oafse/internal/application/port"
	"oafse/internal/domain/model"
	"oafse/internal/infrastructure/metrics"
)

type Worker struct {
	id     string
	parse  port.ParseUseCase
	notify <-chan struct{}
}

func NewWorker(id string, parse port.ParseUseCase, notify <-chan struct{}) *Worker {
	return &Worker{
		id:     id,
		parse:  parse,
		notify: notify,
	}
}

type WorkerPool struct {
	wg          *sync.WaitGroup
	workers     []*Worker
	notifyChans []chan struct{}
	urlRepo     port.URLRepo
}

func NewWorkerPool(cfg *model.CrawlConfig, parse port.ParseUseCase, urlRepo port.URLRepo) *WorkerPool {
	workers := make([]*Worker, cfg.WorkersCount)
	notifyChans := make([]chan struct{}, cfg.WorkersCount)
	for i := range cfg.WorkersCount {
		notifyChans[i] = make(chan struct{}, 1)
		workers[i] = NewWorker(strconv.Itoa(i), parse, notifyChans[i])
	}

	pool := &WorkerPool{
		wg:          &sync.WaitGroup{},
		workers:     workers,
		notifyChans: notifyChans,
		urlRepo:     urlRepo,
	}
	return pool
}

func (p *WorkerPool) Run(ctx context.Context) {
	notify, err := p.urlRepo.Subscribe(ctx)
	if err != nil {
		log.Printf("[ERR] worker pool run: subscribe: %s", err)
		return
	}
	go p.fanOut(ctx, notify)

	for _, worker := range p.workers {
		p.wg.Go(func() {
			worker.run(ctx)
		})
	}
	p.wg.Wait()
	log.Printf("[INFO] parsing is finished")
}

func (p *WorkerPool) fanOut(ctx context.Context, notify <-chan struct{}) {
	for {
		select {
		case <-notify:
			for _, ch := range p.notifyChans {
				select {
				case ch <- struct{}{}:
				default:
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func (w *Worker) run(parent context.Context) {
	for {
		ctx, cancel := context.WithTimeout(parent, 15*time.Second)
		ch := make(chan *port.ParseCmd, 1)

		go func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[ERR] worker run: panic recovered: %v\n%s", r, debug.Stack())
					ch <- nil
				}
			}()

			metrics.ActiveWorkers.Inc()
			defer metrics.ActiveWorkers.Dec()

			cmd, err := w.parse.Execute(ctx, w.id)
			if err != nil {
				log.Printf("[WARN] worker run: %s", err)
			}
			ch <- cmd
		}()

		select {
		case cmd := <-ch:
			cancel()

			if cmd == nil {
				break
			}
			if cmd.SleepFor > 0 {
				timer := time.NewTimer(cmd.SleepFor)
				select {
				case <-timer.C:
				case <-w.notify:
					timer.Stop()
				case <-parent.Done():
					timer.Stop()
					return
				}
			} else {
				select {
				case <-w.notify:
				case <-parent.Done():
					return
				}
			}
		case <-ctx.Done():
			cancel()
			if parent.Err() != nil {
				return
			}
		}
	}
}
