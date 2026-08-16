package scheduler

import (
	"context"
	"sync"
	"time"

	"ejina-microgrid/internal/app"
	"ejina-microgrid/internal/config"
)

// Scheduler runs periodic background tasks: work-order acceptance timeout
// escalation, black-start deadline enforcement, and SMS outbox retry.
type Scheduler struct {
	svc     *app.Service
	cfg     *config.Config
	mu      sync.Mutex
	stopped bool
}

// New creates a Scheduler bound to the given service and config.
func New(svc *app.Service, cfg *config.Config) *Scheduler {
	return &Scheduler{svc: svc, cfg: cfg}
}

// Start launches the scheduler loop in a background goroutine. The loop runs
// until ctx is cancelled. The returned function can be called to wait for
// shutdown, but callers should cancel ctx first.
func (s *Scheduler) Start(ctx context.Context) context.CancelFunc {
	ctx, cancel := context.WithCancel(ctx)
	go s.loop(ctx)
	return cancel
}

// loop runs the periodic tick until ctx is cancelled.
func (s *Scheduler) loop(ctx context.Context) {
	interval := s.cfg.SchedulerInterval
	if interval <= 0 {
		interval = 10 * time.Second
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			s.mu.Lock()
			s.stopped = true
			s.mu.Unlock()
			return
		case <-ticker.C:
			s.tick()
		}
	}
}

// tick executes one round of all background checks. It is exported so tests
// can invoke a single round without waiting for the ticker.
func (s *Scheduler) tick() {
	_, _ = s.svc.EscalateTimedOutWorkOrders()
	_, _ = s.svc.FailExpiredBlackStarts()
	_, _ = s.svc.ResendPendingSMS()
}

// RunOnce executes one round of all background checks synchronously. Useful for
// testing and ad-hoc invocation.
func (s *Scheduler) RunOnce() { s.tick() }

// IsStopped reports whether the scheduler loop has exited.
func (s *Scheduler) IsStopped() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopped
}
