package downstream

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/shopspring/decimal"
)

func TestSimulatedPSPRetriesBeforeAndAfterCommit(t *testing.T) {
	amount := decimal.RequireFromString("10.00")
	for _, after := range []bool{false, true} {
		t.Run(map[bool]string{false: "before commit", true: "after commit"}[after], func(t *testing.T) {
			calls := map[string]int{}
			simulated := NewSimulatedPSP(func(op, key string, attempt int, committed bool) error {
				if committed == after {
					calls[op]++
					if attempt == 1 {
						return ErrPSPTransient
					}
				}
				return nil
			})
			p := NewResilientPSP(simulated)
			p.retry.wait = noWait
			auth, err := p.Authorize(context.Background(), "authorize:payment", amount, "USD")
			if err != nil {
				t.Fatal(err)
			}
			cap, err := p.Capture(context.Background(), "capture:payment", auth, amount, "USD")
			if err != nil {
				t.Fatal(err)
			}
			if len(simulated.results) != 2 || calls["authorize"] == 0 || calls["capture"] == 0 {
				t.Fatalf("results=%v calls=%v", simulated.results, calls)
			}
			again, err := p.Capture(context.Background(), "capture:payment", auth, amount, "USD")
			if err != nil || again != cap {
				t.Fatalf("capture=%s err=%v", again, err)
			}
			if _, err := p.Capture(context.Background(), "capture:payment", auth, decimal.NewFromInt(11), "USD"); !errors.Is(err, ErrPSPDeclined) {
				t.Fatal("accepted changed payload")
			}
			restarted := NewSimulatedPSP(nil)
			auth2, err := restarted.Authorize(context.Background(), "authorize:payment", amount, "USD")
			if err != nil || auth2 != auth {
				t.Fatal("authorization reference not deterministic")
			}
			cap2, err := restarted.Capture(context.Background(), "capture:payment", auth2, amount, "USD")
			if err != nil || cap2 != cap {
				t.Fatal("capture reference not deterministic")
			}
		})
	}
}
func TestPSPDeclineAndBreaker(t *testing.T) {
	amount := decimal.NewFromInt(1)
	calls := 0
	adapter := NewSimulatedPSP(func(string, string, int, bool) error { calls++; return ErrPSPDeclined })
	p := NewResilientPSP(adapter)
	p.retry.wait = noWait
	if _, err := p.Authorize(context.Background(), "decline", amount, "USD"); !errors.Is(err, ErrPSPDeclined) || calls != 1 || len(adapter.results) != 0 {
		t.Fatalf("err=%v calls=%d results=%v", err, calls, adapter.results)
	}
	adapter = NewSimulatedPSP(func(string, string, int, bool) error { return context.DeadlineExceeded })
	p = NewResilientPSP(adapter)
	p.retry.wait = noWait
	now := time.Unix(100, 0)
	p.retry.now = func() time.Time { return now }
	for i := 0; i < 3; i++ {
		if _, err := p.Authorize(context.Background(), "timeout", amount, "USD"); !errors.Is(err, context.DeadlineExceeded) {
			t.Fatal(err)
		}
	}
	if _, err := p.Authorize(context.Background(), "timeout", amount, "USD"); !errors.Is(err, ErrCircuitOpen) {
		t.Fatal(err)
	}
	now = now.Add(p.retry.cooldown)
	adapter.fault = nil
	if _, err := p.Authorize(context.Background(), "recovered", amount, "USD"); err != nil {
		t.Fatal(err)
	}
	if !p.retry.openUntil.IsZero() {
		t.Fatal("successful probe failed to close circuit")
	}
}
func TestPSPValidationAndCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	simulated := NewSimulatedPSP(nil)
	p := NewResilientPSP(simulated)
	if _, err := p.Authorize(ctx, "canceled", decimal.NewFromInt(1), "USD"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := simulated.Authorize(ctx, "canceled", decimal.NewFromInt(1), "USD"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, amount := range []int64{0, -1} {
		if _, err := simulated.Authorize(context.Background(), "invalid", decimal.NewFromInt(amount), "USD"); !errors.Is(err, ErrPSPDeclined) {
			t.Fatal(err)
		}
	}
	if _, err := simulated.Authorize(context.Background(), "", decimal.NewFromInt(1), "USD"); !errors.Is(err, ErrPSPDeclined) {
		t.Fatal(err)
	}
	if _, err := simulated.Capture(context.Background(), "capture", "invalid", decimal.NewFromInt(1), "USD"); !errors.Is(err, ErrPSPDeclined) {
		t.Fatal(err)
	}
	adapter := NewSimulatedPSP(func(string, string, int, bool) error { return ErrPSPTransient })
	p = NewResilientPSP(adapter)
	active, stop := context.WithCancel(context.Background())
	defer stop()
	p.retry.wait = func(ctx context.Context, _ time.Duration) error { stop(); return ctx.Err() }
	if _, err := p.Authorize(active, "cancel-backoff", decimal.NewFromInt(1), "USD"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
