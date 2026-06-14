package worker

import (
	"context"
	"oafse/internal/application/usecase"
	"sync"
)

type Worker struct {
	id int
	parse usecase.ParseUseCase
}

func (w *Worker) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wh.Done()
	for {
		select {
			case <-ctx.Done();
			return
		default:
			if err := w.parse.Execute(ctx); err != nil
		}
	}
}

