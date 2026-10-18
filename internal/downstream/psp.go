package downstream

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrPSPTransient = errors.New("temporary simulated PSP failure")
var ErrPSPDeclined = errors.New("simulated PSP declined")

type PSP interface {
	Authorize(context.Context, string, decimal.Decimal, string) (string, error)
	Capture(context.Context, string, string, decimal.Decimal, string) (string, error)
}

// PSPFault is test-only injection. A post-commit failure models a lost response.
type PSPFault func(operation, key string, attempt int, afterCommit bool) error

type SimulatedPSP struct {
	mu       sync.Mutex
	results  map[string]string
	attempts map[string]int
	fault    PSPFault
}

func NewSimulatedPSP(fault PSPFault) *SimulatedPSP {
	return &SimulatedPSP{results: make(map[string]string), attempts: make(map[string]int), fault: fault}
}
func (p *SimulatedPSP) Authorize(ctx context.Context, key string, amount decimal.Decimal, currency string) (string, error) {
	return p.operation(ctx, "authorize", key, "", amount, currency)
}
func (p *SimulatedPSP) Capture(ctx context.Context, key, authorization string, amount decimal.Decimal, currency string) (string, error) {
	if _, err := uuid.Parse(authorization); err != nil {
		return "", ErrPSPDeclined
	}
	return p.operation(ctx, "capture", key, authorization, amount, currency)
}
func (p *SimulatedPSP) operation(ctx context.Context, operation, key, authorization string, amount decimal.Decimal, currency string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if key == "" || domain.ValidateAmountCurrency(amount, currency) != nil {
		return "", ErrPSPDeclined
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	identity := operation + ":" + key
	// Include the immutable payload in the deterministic reference. Reusing a
	// key with another payload is a decline, not a different successful effect.
	ref := uuid.NewSHA1(uuid.NameSpaceURL, []byte(identity+":"+authorization+":"+amount.String()+":"+currency)).String()
	if existing, ok := p.results[identity]; ok {
		if existing != ref {
			return "", ErrPSPDeclined
		}
		return existing, nil
	}
	p.attempts[identity]++
	attempt := p.attempts[identity]
	if p.fault != nil {
		if err := p.fault(operation, key, attempt, false); err != nil {
			return "", err
		}
	}
	p.results[identity] = ref
	if p.fault != nil {
		if err := p.fault(operation, key, attempt, true); err != nil {
			return "", err
		}
	}
	return ref, nil
}

type ResilientPSP struct {
	adapter PSP
	retry   *retryPolicy
}

func NewResilientPSP(adapter PSP) *ResilientPSP {
	return &ResilientPSP{adapter: adapter, retry: defaultRetryPolicy()}
}
func (p *ResilientPSP) Authorize(ctx context.Context, key string, amount decimal.Decimal, currency string) (string, error) {
	return p.run(ctx, func(ctx context.Context) (string, error) { return p.adapter.Authorize(ctx, key, amount, currency) })
}
func (p *ResilientPSP) Capture(ctx context.Context, key, auth string, amount decimal.Decimal, currency string) (string, error) {
	return p.run(ctx, func(ctx context.Context) (string, error) { return p.adapter.Capture(ctx, key, auth, amount, currency) })
}
func (p *ResilientPSP) run(ctx context.Context, call func(context.Context) (string, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	epoch, err := p.retry.acquire()
	if err != nil {
		return "", err
	}
	failed := false
	defer func() {
		if ctx.Err() != nil {
			p.retry.abandon(epoch)
		} else {
			p.retry.finish(epoch, failed)
		}
	}()
	for i := 0; i < p.retry.attempts; i++ {
		attemptCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		ref, err := call(attemptCtx)
		cancel()
		if err == nil {
			failed = false
			if _, err := uuid.Parse(ref); err != nil {
				return "", fmt.Errorf("invalid PSP reference: %w", err)
			}
			return ref, nil
		}
		failed = errors.Is(err, ErrPSPTransient) || errors.Is(err, context.DeadlineExceeded)
		if !failed {
			return "", err
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if i+1 == p.retry.attempts {
			return "", err
		}
		if err := p.retry.wait(ctx, p.retry.delay(i)); err != nil {
			return "", err
		}
	}
	return "", ErrPSPTransient
}
