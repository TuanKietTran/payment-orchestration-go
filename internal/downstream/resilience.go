package downstream

import (
	"context"
	"errors"
	"math/rand/v2"
	"sync"
	"time"
)

var ErrCircuitOpen = errors.New("downstream circuit open")

// retryPolicy bounds each logical operation to three attempts. The breaker
// counts exhausted operations, not individual attempts. A single half-open
// probe may enter after cooldown; other callers fail fast until it completes.
type retryPolicy struct {
	attempts                 int
	base, maxDelay, cooldown time.Duration
	threshold                int
	now                      func() time.Time
	wait                     func(context.Context, time.Duration) error
	jitter                   func(time.Duration) time.Duration
	mu                       sync.Mutex
	failures                 int
	openUntil                time.Time
	probing                  bool
	epoch                    uint64
}

func defaultRetryPolicy() *retryPolicy {
	return &retryPolicy{attempts: 3, base: 25 * time.Millisecond, maxDelay: 250 * time.Millisecond, cooldown: 5 * time.Second, threshold: 3, now: time.Now, wait: waitContext, jitter: func(d time.Duration) time.Duration { return d/2 + time.Duration(rand.Int64N(int64(d/2)+1)) }}
}
func waitContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (p *retryPolicy) acquire() (uint64, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.openUntil.IsZero() {
		return p.epoch, nil
	}
	if p.now().Before(p.openUntil) || p.probing {
		return 0, ErrCircuitOpen
	}
	p.probing = true
	p.epoch++
	return p.epoch, nil
}
func (p *retryPolicy) finish(epoch uint64, failed bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	// Ignore late results from operations admitted before this circuit opened.
	if epoch != p.epoch {
		return
	}
	if !failed {
		p.failures = 0
		p.openUntil = time.Time{}
		p.probing = false
		return
	}
	p.failures++
	if p.failures >= p.threshold || p.probing {
		p.openUntil = p.now().Add(p.cooldown)
		p.probing = false
		p.epoch++
	}
}
func (p *retryPolicy) abandon(epoch uint64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if epoch == p.epoch && p.probing {
		p.probing = false
		p.openUntil = p.now().Add(p.cooldown)
		p.epoch++
	}
}
func (p *retryPolicy) delay(attempt int) time.Duration {
	delay := p.base
	for i := 0; i < attempt && delay < p.maxDelay; i++ {
		delay *= 2
	}
	if delay > p.maxDelay {
		delay = p.maxDelay
	}
	return p.jitter(delay)
}
