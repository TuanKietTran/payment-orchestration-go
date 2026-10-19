package playback

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/TuanKietTran/payment-orchestration-go/internal/api"
	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type r1Ledger struct {
	mu    sync.Mutex
	keys  map[string]string
	posts int
}

func (l *r1Ledger) Post(_ context.Context, key, _, _ string, _ decimal.Decimal, _ string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if id, ok := l.keys[key]; ok {
		return id, nil
	}
	id := uuid.NewString()
	l.keys[key] = id
	l.posts++
	return id, nil
}

func TestR1TenThousandProductionHTTPRequests(t *testing.T) {
	repo := repository.NewInMemoryPayment()
	ledger := &r1Ledger{keys: map[string]string{}}
	process := command.NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger)
	h := api.NewHandlers(command.NewCreatePaymentHandler(repo), query.NewGetPaymentHandler(repo), process)
	router := chi.NewRouter()
	var mu sync.Mutex
	attempts := map[string]int{}
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/v1/payments" {
				key := r.URL.Query().Get("fault")
				if key != "" {
					mu.Lock()
					attempts[key]++
					n := attempts[key]
					mu.Unlock()
					if n == 1 {
						switch key[len(key)-1] {
						case '0':
							http.Error(w, "transient", 503)
							return
						case '1':
							http.Error(w, "transient", 429)
							return
						case '2':
							<-time.After(400 * time.Millisecond)
							next.ServeHTTP(w, r)
							return
						}
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	})
	router.Mount("/v1", h.Routes())
	srv := httptest.NewServer(router)
	defer srv.Close()
	const n = 10000
	calls := make([]Call, n)
	keys := make([]string, n)
	for i := range calls {
		unique := i
		if i%50 == 49 {
			unique = i - 1
		}
		key := fmt.Sprintf("r1-%05d", unique)
		keys[i] = key
		suffix := byte('3')
		if unique%100 == 0 {
			suffix = '0'
		} else if unique%100 == 1 {
			suffix = '1'
		} else if unique%100 == 2 {
			suffix = '2'
		}
		calls[i] = Call{Method: http.MethodPost, Path: "/v1/payments?fault=" + key + string(suffix), Body: []byte(fmt.Sprintf(`{"idempotency_key":%q,"source_account":"src","dest_account":"dst","amount":"1.00","currency":"USD"}`, key))}
	}
	ctx, cancel := context.WithTimeout(context.Background(), Window)
	defer cancel()
	start := time.Now()
	created, err := Play(ctx, srv.Client(), srv.URL, calls, Policy{MaxAttempts: 6, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Second, AttemptTimeout: 250 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) >= Window {
		t.Fatalf("create phase exceeded window: %s", time.Since(start))
	}
	ids := make(map[string]string)
	processCalls := make([]Call, 0, len(calls))
	for i, o := range created {
		if o.Status != 200 {
			t.Fatalf("create %d status=%d attempts=%d err=%v body=%s", i, o.Status, o.Attempts, o.Err, o.Body)
		}
		var result struct {
			ID string `json:"payment_id"`
		}
		if err := json.Unmarshal(o.Body, &result); err != nil {
			t.Fatal(err)
		}
		if old := ids[keys[i]]; old != "" && old != result.ID {
			t.Fatalf("duplicate key %s ids %s != %s", keys[i], old, result.ID)
		}
		ids[keys[i]] = result.ID
	}
	for key, id := range ids {
		_ = key
		processCalls = append(processCalls, Call{Method: http.MethodPost, Path: "/v1/payments/" + id + "/process"})
	}
	processed, err := Play(ctx, srv.Client(), srv.URL, processCalls, Policy{MaxAttempts: 6, BaseDelay: time.Millisecond, MaxDelay: 2 * time.Second, AttemptTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	for i, o := range processed {
		if o.Status != 200 {
			t.Fatalf("process %d status=%d attempts=%d err=%v body=%s", i, o.Status, o.Attempts, o.Err, o.Body)
		}
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if ledger.posts != len(ids) || len(ledger.keys) != len(ids) {
		t.Fatalf("effects=%d unique=%d payments=%d", ledger.posts, len(ledger.keys), len(ids))
	}
}
