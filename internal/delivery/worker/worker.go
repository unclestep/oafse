package worker

import (
	"context"
	"sync"
	"time"

	"oafse/internal/application/port"
)

type Worker struct {
	id    string
	parse port.ParseUseCase
}

func (w *Worker) run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
mainloop:
	for {
		select {
		case <-ctx.Done():
			return
		default:
			if cmd, _ := w.parse.Execute(ctx, w.id); cmd != nil {
				switch cmd.Directive {
				case port.DirectiveSleep:
					time.Sleep(cmd.SleepFor)
				case port.DirectiveStop:
					break mainloop
				default:
				}
			}
		}
	}
}
