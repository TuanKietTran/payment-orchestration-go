package playback

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/example/payment-orchestration/internal/api"
	"github.com/example/payment-orchestration/internal/command"
	"github.com/example/payment-orchestration/internal/query"
	"github.com/example/payment-orchestration/internal/repository"
	"github.com/go-chi/chi/v5"
)

func failFirst(next http.Handler) http.Handler {
	var mu sync.Mutex
	seen := map[string]int{}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		mu.Lock()
		seen[string(body)]++
		n := seen[string(body)]
		mu.Unlock()
		if n == 1 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func TestThousandPerMinuteAndPlayback(t *testing.T) {
	repo := repository.NewInMemoryPayment()
	h := api.NewHandlers(command.NewCreatePaymentHandler(repo), query.NewGetPaymentHandler(repo))
	router := chi.NewRouter()
	router.Use(failFirst)
	router.Mount("/v1", h.Routes())
	srv := httptest.NewServer(router)
	t.Cleanup(srv.Close)

	calls := make([]Call, RequestsPerMinute)
	for i := range calls {
		calls[i] = Call{
			Method: http.MethodPost,
			Path:   "/v1/payments",
			Body: []byte(fmt.Sprintf(
				`{"idempotency_key":"p-%d","source_account":"src","dest_account":"dst","amount":"1","currency":"USD"}`, i)),
		}
	}
	start := time.Now()
	first, err := Play(context.Background(), srv.Client(), srv.URL, calls, DefaultPolicy())
	elapsed := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	if elapsed >= Window {
		t.Fatalf("handled %d requests in %s, want under %s", len(calls), elapsed, Window)
	}
	t.Logf("first play %d requests in %s", len(calls), elapsed)
	ids := make([]string, len(first))
	for i, o := range first {
		if o.Err != nil || o.Status != http.StatusOK || o.Attempts != 2 {
			t.Fatalf("call %d status %d attempts %d err %v body %s", i, o.Status, o.Attempts, o.Err, o.Body)
		}
		var created struct {
			PaymentID string `json:"payment_id"`
		}
		if err := json.Unmarshal(o.Body, &created); err != nil || created.PaymentID == "" {
			t.Fatalf("call %d body %s", i, o.Body)
		}
		ids[i] = created.PaymentID
	}

	second, err := Play(context.Background(), srv.Client(), srv.URL, calls, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	for i, o := range second {
		if o.Attempts != 1 || !IsIdempotentReplay(o.Status, o.Body) || !bytesContains(o.Body, ids[i]) {
			t.Fatalf("playback %d status %d body %s", i, o.Status, o.Body)
		}
	}
	got, err := Play(context.Background(), srv.Client(), srv.URL, []Call{{
		Method: http.MethodGet, Path: "/v1/payments/" + ids[0],
	}}, DefaultPolicy())
	if err != nil {
		t.Fatal(err)
	}
	if got[0].Status != http.StatusOK || !bytesContains(got[0].Body, ids[0]) {
		t.Fatalf("get after playback %+v", got[0])
	}
}

func bytesContains(b []byte, s string) bool {
	return len(s) > 0 && len(b) >= len(s) && (string(b) == s || contains(string(b), s))
}

func contains(hay, needle string) bool {
	return len(needle) == 0 || (len(hay) >= len(needle) && (hay == needle || len(hay) > 0 && (func() bool {
		for i := 0; i+len(needle) <= len(hay); i++ {
			if hay[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()))
}
