package worker_test

import (
	"context"
	"errors"
	"sync"
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

// MockURLRepo stubs port.URLRepo. Only Subscribe is exercised by worker.go:
// each call mimics a fresh Redis pub/sub subscription, so every worker gets
// its own channel and publish() fans a notification out to all of them.
type MockURLRepo struct {
	mock.Mock

	mu   sync.Mutex
	subs []chan struct{}
}

func newMockURLRepo() *MockURLRepo {
	return &MockURLRepo{}
}

func (m *MockURLRepo) Start(ctx context.Context, url string) error { return nil }
func (m *MockURLRepo) StartHealthChecking(ctx context.Context, cfg *model.CrawlConfig) {
}
func (m *MockURLRepo) TakeOn(ctx context.Context, workerID string) (*model.URL, error) {
	return nil, nil
}
func (m *MockURLRepo) PushURLs(ctx context.Context, urls []*model.URL) ([]*model.URL, error) {
	return nil, nil
}
func (m *MockURLRepo) RetryURL(ctx context.Context, url string, retryAt time.Time) error {
	return nil
}
func (m *MockURLRepo) MarkProcessed(ctx context.Context, url string, status model.CrawlStatus) error {
	return nil
}
func (m *MockURLRepo) GetCrawlMetadata(ctx context.Context) (*port.CrawlMetadata, error) {
	return &port.CrawlMetadata{}, nil
}
func (m *MockURLRepo) ResetCrawlCache(ctx context.Context) error { return nil }

func (m *MockURLRepo) Subscribe(ctx context.Context) (<-chan struct{}, error) {
	ch := make(chan struct{}, 1)
	m.mu.Lock()
	m.subs = append(m.subs, ch)
	m.mu.Unlock()
	return ch, nil
}

func (m *MockURLRepo) publish() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, ch := range m.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func newPool(n int, uc port.ParseUseCase, urlRepo port.URLRepo) *worker.WorkerPool {
	cfg := &model.CrawlConfig{WorkersCount: n}
	return worker.NewWorkerPool(cfg, uc, urlRepo)
}

// Scenario 1: 1 URL in queue, n > 1 workers.
// One worker picks up the URL (nil cmd) and loops.
// The other n-1 workers find the queue empty but the URL still in-processing,
// so they receive a short sleep deadline (mirrors parse.go: UnprocessedCount > 0 → sleep).
// On the next round all workers find the queue fully drained and idle-wait
// (no work anywhere, no notification arrives, so they just sit on it).
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
			Return(&port.ParseCmd{SleepFor: sleepFor}, nil).Once()
	}
	// Second round: everything is drained, workers idle-wait on notify.
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{}, nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	newPool(n, uc, urlRepo).Run(ctx)

	uc.AssertExpectations(t)
	uc.AssertNumberOfCalls(t, "Execute", 2*n)
}

// Scenario 2: Empty queue — all workers idle-wait immediately.
func TestWorkerPoolEmptyQueueAllIdle(t *testing.T) {
	const n = 4

	uc := new(MockParseUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{}, nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	newPool(n, uc, urlRepo).Run(ctx)

	uc.AssertNumberOfCalls(t, "Execute", n)
}

// Scenario 3: All URLs in retry — workers sleep until the retry deadline, then idle-wait.
func TestWorkerPoolAllInRetrySleepThenIdle(t *testing.T) {
	const (
		n        = 3
		sleepFor = 100 * time.Millisecond
	)

	uc := new(MockParseUseCase)
	for range n {
		uc.On("Execute", mock.Anything, mock.Anything).
			Return(&port.ParseCmd{SleepFor: sleepFor}, nil).Once()
	}
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{}, nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	start := time.Now()
	newPool(n, uc, urlRepo).Run(ctx)

	assert.GreaterOrEqual(t, time.Since(start), sleepFor)
	uc.AssertNumberOfCalls(t, "Execute", n*2)
}

// Scenario 4: Context cancelled while workers are sleeping — pool exits without waiting SleepFor.
func TestWorkerPoolContextCancelledDuringSleep(t *testing.T) {
	const n = 4

	uc := new(MockParseUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).
		Return(&port.ParseCmd{SleepFor: 10 * time.Second}, nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	newPool(n, uc, urlRepo).Run(ctx)

	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

// Scenario 5: Context cancelled while workers are idle-waiting (no SleepFor at all)
// — pool must still exit promptly via the parent.Done() case.
func TestWorkerPoolContextCancelledDuringIdleWait(t *testing.T) {
	const n = 4

	uc := new(MockParseUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).Return(&port.ParseCmd{}, nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	newPool(n, uc, urlRepo).Run(ctx)

	assert.Less(t, time.Since(start), 500*time.Millisecond)
}

// Scenario 6: A worker idle-waiting on notify wakes up as soon as a queue
// insert is published, instead of waiting for any timer.
func TestWorkerPoolWakesOnNotify(t *testing.T) {
	const n = 1

	var calls atomic.Int32

	uc := new(MockParseUseCase)
	uc.On("Execute", mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) { calls.Add(1) }).
		Return(&port.ParseCmd{SleepFor: 10 * time.Second}, nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go newPool(n, uc, urlRepo).Run(ctx)

	assert.Eventually(t, func() bool { return calls.Load() >= 1 }, time.Second, 5*time.Millisecond)

	urlRepo.publish()

	assert.Eventually(t, func() bool { return calls.Load() >= 2 }, time.Second, 5*time.Millisecond)
}

// Scenario 7: N workers, N URLs — all must run concurrently.
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
		Return((*port.ParseCmd)(nil), nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	newPool(n, uc, urlRepo).Run(ctx)

	assert.Equal(t, int32(n), peakConcurrent.Load(), "all workers must run concurrently")
}

// Scenario 8: Execute returns an error — worker silently ignores it and continues looping.
func TestWorkerPoolExecuteErrorContinues(t *testing.T) {
	const n = 2

	uc := new(MockParseUseCase)
	for range n {
		uc.On("Execute", mock.Anything, mock.Anything).
			Return((*port.ParseCmd)(nil), errors.New("transient failure")).Once()
	}
	uc.On("Execute", mock.Anything, mock.Anything).
		Return(&port.ParseCmd{SleepFor: 10 * time.Second}, nil)

	urlRepo := newMockURLRepo()

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	newPool(n, uc, urlRepo).Run(ctx)

	uc.AssertNumberOfCalls(t, "Execute", n*2)
}
