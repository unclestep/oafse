package worker_test

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"oafse/internal/application/port"
	"oafse/internal/delivery/worker"
	"oafse/internal/domain/model"
)

// MockParseUseCase is a testify/mock implementation of port.ParseUseCase.
type MockParseUseCase struct {
	mock.Mock
}

func (m *MockParseUseCase) Execute(ctx context.Context, workerID string) (*port.ParseCmd, error) {
	args := m.Called(ctx, workerID)
	cmd, _ := args.Get(0).(*port.ParseCmd)
	return cmd, args.Error(1)
}

func newPool(n int, uc port.ParseUseCase) *worker.WorkerPool {
	cfg := &model.CrawlConfig{WorkersCount: n}
	return worker.NewWorkerPool(cfg, uc)
}

// Scenario 1: 1 URL in queue, n > 1 workers.
// One worker picks up the URL (nil cmd) and loops.
// The other n-1 workers find the queue empty but the URL still in-processing,
// so they receive DirectiveSleep (mirrors parse.go: UnprocessedCount > 0 → sleep).
// On the next round all workers see an empty, fully-processed queue and stop.
func TestWorkerPoolSingleURLMultipleWorkers(t *testing.T) {
	const (
		n        = 5
		sleepFor = 50 * time.Millisecond
	)

	uc := new(MockParseUseCase)
	// One worker processes the URL.
	uc.On("Execute", mock.Anything, mock.Anything).Return((*port.ParseCmd)(nil), nil).Once()
	// The remaining n-1 workers sleep while the URL is in-processing.
	for range n - 1 {
		uc.On("Execute", mock.Anything, mock.Anything).
			Return(&port.ParseCmd{Directive: port.DirectiveSleep, SleepFor: sleepFor}, nil).Once()
	}
	// Second round: all n workers stop (queue fully processed, no new links).
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{Directive: port.DirectiveStop}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	newPool(n, uc).Run(ctx)

	uc.AssertExpectations(t)
	uc.AssertNumberOfCalls(t, "Execute", 2*n)
}

// Scenario 2: Empty queue — all workers stop immediately.
func TestWorkerPoolEmptyQueueAllStop(t *testing.T) {
	const n = 4

	uc := new(MockParseUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{Directive: port.DirectiveStop}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	newPool(n, uc).Run(ctx)

	uc.AssertNumberOfCalls(t, "Execute", n)
}

// Scenario 3: All URLs in retry — workers sleep, then stop.
func TestWorkerPoolAllInRetrySleepThenStop(t *testing.T) {
	const (
		n        = 3
		sleepFor = 100 * time.Millisecond
	)

	uc := new(MockParseUseCase)
	for range n {
		uc.On("Execute", mock.Anything, mock.Anything).
			Return(&port.ParseCmd{Directive: port.DirectiveSleep, SleepFor: sleepFor}, nil).Once()
	}
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{Directive: port.DirectiveStop}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	start := time.Now()
	newPool(n, uc).Run(ctx)

	assert.GreaterOrEqual(t, time.Since(start), sleepFor)
	uc.AssertNumberOfCalls(t, "Execute", n*2)
}

// Scenario 4: Context cancelled while workers are sleeping — pool exits without waiting SleepFor.
func TestWorkerPoolContextCancelledDuringSleep(t *testing.T) {
	const n = 4

	uc := new(MockParseUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).
		Return(&port.ParseCmd{Directive: port.DirectiveSleep, SleepFor: 10 * time.Second}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	newPool(n, uc).Run(ctx)

	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

// Scenario 5: N workers, N URLs — all must run concurrently.
// Peak in-flight Execute calls must equal n.
func TestWorkerPoolConcurrentProcessing(t *testing.T) {
	const n = 5

	var (
		current        atomic.Int32
		peakConcurrent atomic.Int32
	)

	uc := new(MockParseUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			cur := current.Add(1)
			for {
				peak := peakConcurrent.Load()
				if cur <= peak {
					break
				}
				if peakConcurrent.CompareAndSwap(peak, cur) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			current.Add(-1)
		}).
		Return(&port.ParseCmd{Directive: port.DirectiveStop}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	newPool(n, uc).Run(ctx)

	assert.Equal(t, int32(n), peakConcurrent.Load(), "all workers must run concurrently")
}

// Scenario 6: Execute returns an error — worker silently ignores it and continues looping.
func TestWorkerPoolExecuteErrorContinues(t *testing.T) {
	const n = 2

	uc := new(MockParseUseCase)
	for range n {
		uc.On("Execute", mock.Anything, mock.Anything).
			Return((*port.ParseCmd)(nil), errors.New("transient failure")).Once()
	}
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{Directive: port.DirectiveStop}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	newPool(n, uc).Run(ctx)

	uc.AssertNumberOfCalls(t, "Execute", n*2)
}
