package downstream

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func noWait(ctx context.Context, _ time.Duration) error { return ctx.Err() }

func TestLedgerRetriesPreserveExactPayloadAfterAmbiguousCommit(t *testing.T) {
	var payloads []string
	var mu sync.Mutex
	ref := uuid.NewString()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		mu.Lock()
		payloads = append(payloads, string(b))
		count := len(payloads)
		mu.Unlock()
		if count == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = io.WriteString(w, `{"transaction_id":"`+ref+`"}`)
	}))
	defer server.Close()
	client, err := NewHTTP(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	var delays []time.Duration
	client.retry.jitter = func(d time.Duration) time.Duration { return d }
	client.retry.wait = func(_ context.Context, d time.Duration) error { delays = append(delays, d); return nil }
	got, err := client.Post(context.Background(), "stable-key", uuid.NewString(), uuid.NewString(), decimal.RequireFromString("1.23"), "USD")
	if err != nil || got != ref {
		t.Fatalf("got=%s err=%v", got, err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(payloads) != 2 || payloads[0] != payloads[1] || !strings.Contains(payloads[0], `"idempotency_key":"stable-key"`) {
		t.Fatalf("payloads=%v", payloads)
	}
	if len(delays) != 1 || delays[0] != 25*time.Millisecond {
		t.Fatalf("delays=%v", delays)
	}
}

func TestHTTPRetryClassificationAndBreaker(t *testing.T) {
	for _, tc := range []struct{ status, attempts int }{{400, 1}, {404, 1}, {409, 1}, {408, 3}, {429, 3}, {500, 3}, {503, 3}} {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { calls.Add(1); w.WriteHeader(tc.status) }))
			defer server.Close()
			client, _ := NewHTTP(server.URL)
			client.retry.wait = noWait
			now := time.Unix(100, 0)
			client.retry.now = func() time.Time { return now }
			for i := 0; i < 3; i++ {
				if err := client.request(context.Background(), http.MethodGet, "/", nil, nil); err == nil {
					t.Fatal("expected failure")
				}
			}
			if calls.Load() != int32(3*tc.attempts) {
				t.Fatalf("calls=%d", calls.Load())
			}
			err := client.request(context.Background(), http.MethodGet, "/", nil, nil)
			if tc.attempts == 3 {
				if !errors.Is(err, ErrCircuitOpen) || calls.Load() != 9 {
					t.Fatalf("circuit err=%v calls=%d", err, calls.Load())
				}
				now = now.Add(5 * time.Second)
				if err := client.request(context.Background(), http.MethodGet, "/", nil, nil); errors.Is(err, ErrCircuitOpen) {
					t.Fatal("probe rejected")
				}
				if calls.Load() != 12 {
					t.Fatalf("probe calls=%d", calls.Load())
				}
			} else if errors.Is(err, ErrCircuitOpen) {
				t.Fatal("client errors opened circuit")
			}
		})
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTransportTimeoutRetriesAndCancellation(t *testing.T) {
	client, _ := NewHTTP("http://localhost")
	calls := 0
	client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) { calls++; return nil, context.DeadlineExceeded })
	client.retry.wait = noWait
	if err := client.request(context.Background(), "GET", "/", nil, nil); err == nil || calls != 3 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	client.retry.wait = func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
	calls = 0
	if err := client.request(ctx, "GET", "/", nil, nil); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	if err := client.request(ctx, "GET", "/", nil, nil); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("already canceled err=%v calls=%d", err, calls)
	}
}

func TestBreakerSingleProbeAndStaleResults(t *testing.T) {
	p := defaultRetryPolicy()
	now := time.Unix(100, 0)
	p.now = func() time.Time { return now }
	stale, _ := p.acquire()
	for i := 0; i < 3; i++ {
		epoch, err := p.acquire()
		if err != nil {
			t.Fatal(err)
		}
		p.finish(epoch, true)
	}
	p.finish(stale, false)
	if _, err := p.acquire(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatalf("stale result reset circuit: %v", err)
	}
	now = now.Add(p.cooldown)
	probe, err := p.acquire()
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := p.acquire(); !errors.Is(err, ErrCircuitOpen) {
				t.Errorf("parallel probe admitted: %v", err)
			}
		}()
	}
	wg.Wait()
	p.finish(stale, false)
	if _, err := p.acquire(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatal("stale completion reset in-flight probe")
	}
	p.finish(probe, false)
	if _, err := p.acquire(); err != nil {
		t.Fatal("successful probe did not close circuit")
	}
	// Canceled probes neither heal an outage nor retain the probe lock forever.
	for i := 0; i < 3; i++ {
		epoch, _ := p.acquire()
		p.finish(epoch, true)
	}
	now = now.Add(p.cooldown)
	probe, err = p.acquire()
	if err != nil {
		t.Fatal(err)
	}
	p.abandon(probe)
	if _, err := p.acquire(); !errors.Is(err, ErrCircuitOpen) {
		t.Fatal("canceled probe closed circuit")
	}
	now = now.Add(p.cooldown)
	if _, err := p.acquire(); err != nil {
		t.Fatal("canceled probe retained lock")
	}
}

func TestDelayBoundsAndWaitCancellation(t *testing.T) {
	p := defaultRetryPolicy()
	for attempt := 0; attempt < 20; attempt++ {
		d := p.delay(attempt)
		if d < 12*time.Millisecond || d > p.maxDelay {
			t.Fatalf("delay=%s", d)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitContext(ctx, time.Hour); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if err := waitContext(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
}

func TestHTTPValidationAndBoundedResponses(t *testing.T) {
	for _, base := range []string{"", ":bad", "file://localhost", "http://user:fake@localhost", "http://localhost?query=1", "http://localhost/#fragment"} {
		if _, err := NewHTTP(base); err == nil {
			t.Fatalf("accepted %q", base)
		}
	}
	client, _ := NewHTTP("http://localhost")
	client.retry.wait = noWait
	if err := client.request(context.Background(), "POST", "/", make(chan int), nil); err == nil {
		t.Fatal("accepted unencodable body")
	}
	if err := client.request(context.Background(), "bad method", "/", nil, nil); err == nil {
		t.Fatal("accepted invalid method")
	}
	for _, payload := range []string{strings.Repeat("x", (1<<20)+1), "invalid json"} {
		client.client.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(payload)), Header: make(http.Header)}, nil
		})
		var out map[string]string
		if err := client.request(context.Background(), "GET", "/", nil, &out); err == nil {
			t.Fatal("accepted invalid response")
		}
	}
	if _, err := client.Post(context.Background(), "k", "invalid", uuid.NewString(), decimal.NewFromInt(1), "USD"); err == nil {
		t.Fatal("accepted invalid source")
	}
	if _, err := client.Post(context.Background(), "k", uuid.NewString(), "invalid", decimal.NewFromInt(1), "USD"); err == nil {
		t.Fatal("accepted invalid destination")
	}
}
