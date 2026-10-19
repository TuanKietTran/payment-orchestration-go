package playback

import (
	"bytes"
	"context"
	"errors"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

// RequestsPerMinute is the throughput the playback client must finish inside Window.
const (
	RequestsPerMinute = 10000
	Window            = time.Minute
)

// Call is one recorded HTTP request. Playback sends the same value again.
type Call struct {
	Method string
	Path   string
	Body   []byte
}

// Outcome is the last attempt for a call.
type Outcome struct {
	Status   int
	Body     []byte
	Attempts int
	Err      error
}

// Policy controls retries. Only transport errors and HTTP 408, 429, and 5xx are retried.
type Policy struct {
	MaxAttempts    int
	BaseDelay      time.Duration
	MaxDelay       time.Duration
	AttemptTimeout time.Duration
}

func DefaultPolicy() Policy {
	return Policy{MaxAttempts: 6, BaseDelay: 50 * time.Millisecond, MaxDelay: 2 * time.Second, AttemptTimeout: 5 * time.Second}
}

func Retryable(status int) bool {
	return status == http.StatusRequestTimeout || status == http.StatusTooManyRequests || status >= 500
}

func IsIdempotentReplay(status int, body []byte) bool {
	return status == http.StatusOK && bytes.Contains(body, []byte("payment_id"))
}

// Play sends calls in order. A later Play of the same slice is a playback.
func Play(ctx context.Context, client *http.Client, baseURL string, calls []Call, policy Policy) ([]Outcome, error) {
	if client == nil {
		client = http.DefaultClient
	}
	if policy.MaxAttempts < 1 {
		policy.MaxAttempts = 1
	}
	out := make([]Outcome, len(calls))
	const workers = 16
	jobs := make(chan int)
	var wg sync.WaitGroup
	var firstErr error
	var errOnce sync.Once
	for worker := 0; worker < workers && worker < len(calls); worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range jobs {
				outcome, err := playOne(ctx, client, baseURL, calls[i], policy)
				out[i] = outcome
				if err != nil {
					errOnce.Do(func() { firstErr = err })
				}
			}
		}()
	}
	for i := range calls {
		jobs <- i
	}
	close(jobs)
	wg.Wait()
	return out, firstErr
}

func playOne(ctx context.Context, client *http.Client, baseURL string, call Call, policy Policy) (Outcome, error) {
	var last Outcome
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return last, err
		}
		if attempt > 1 && policy.BaseDelay > 0 {
			capDelay := policy.MaxDelay
			if capDelay <= 0 {
				capDelay = 2 * time.Second
			}
			delayCap := policy.BaseDelay
			for n := 1; n < attempt-1 && delayCap < capDelay; n++ {
				delayCap *= 2
			}
			if delayCap > capDelay {
				delayCap = capDelay
			}
			delay := time.Duration(rand.Int63n(int64(delayCap) + 1))
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return last, ctx.Err()
			case <-timer.C:
			}
		}
		attemptCtx := ctx
		cancel := func() {}
		if policy.AttemptTimeout > 0 {
			attemptCtx, cancel = context.WithTimeout(ctx, policy.AttemptTimeout)
		}
		req, err := http.NewRequestWithContext(attemptCtx, call.Method, baseURL+call.Path, bytes.NewReader(call.Body))
		if err != nil {
			cancel()
			return last, err
		}
		if len(call.Body) > 0 {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := client.Do(req)
		last.Attempts = attempt
		if err != nil {
			cancel()
			last.Err = err
			last.Status = 0
			if ctx.Err() != nil {
				return last, ctx.Err()
			}
			if attempt == policy.MaxAttempts || !retryableErr(err) {
				return last, nil
			}
			continue
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		cancel()
		if ctx.Err() != nil {
			return last, ctx.Err()
		}
		if readErr != nil {
			last.Err = readErr
			last.Status = resp.StatusCode
			if attempt == policy.MaxAttempts || !retryableErr(readErr) {
				return last, nil
			}
			continue
		}
		last.Status = resp.StatusCode
		last.Body = body
		last.Err = nil
		if !Retryable(resp.StatusCode) || attempt == policy.MaxAttempts {
			return last, nil
		}
	}
	return last, nil
}

func retryableErr(err error) bool {
	return err != nil && !errors.Is(err, context.Canceled)
}
