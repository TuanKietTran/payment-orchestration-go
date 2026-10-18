package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
)

// listReconSetup posts through its own simulated ledger; reader, when non-nil,
// is what reconciliation consults instead.
func listReconSetup(reader downstream.LedgerReader) http.Handler {
	repo := repository.NewInMemoryPayment()
	ledger := downstream.NewSimulatedLedger()
	if reader == nil {
		reader = ledger
	}
	h := NewHandlers(command.NewCreatePaymentHandler(repo), query.NewGetPaymentHandler(repo),
		command.NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger)).
		WithListing(repo, query.NewReconcileHandler(repo, reader))
	return h.Routes()
}

func do(h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, path, strings.NewReader(body)))
	return rec
}

func seed(t *testing.T, h http.Handler, n int) {
	t.Helper()
	for i := 0; i < n; i++ {
		body := fmt.Sprintf(`{"idempotency_key":"k%d","source_account":"a%d","dest_account":"b%d","amount":"10.00","currency":"USD"}`, i, i, i)
		rec := do(h, http.MethodPost, "/payments", body)
		var c struct {
			PaymentID string `json:"payment_id"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &c); err != nil || c.PaymentID == "" {
			t.Fatalf("create: %d %s", rec.Code, rec.Body)
		}
		if rec := do(h, http.MethodPost, "/payments/"+c.PaymentID+"/process", ""); rec.Code != 200 {
			t.Fatalf("process: %d %s", rec.Code, rec.Body)
		}
	}
}

func TestListPaginatesWithoutGapsOrDuplicates(t *testing.T) {
	h := listReconSetup(nil)
	seed(t, h, 5)
	seen := map[string]bool{}
	after := ""
	for pages := 0; pages < 10; pages++ {
		path := "/payments?limit=2"
		if after != "" {
			path += "&after=" + after
		}
		rec := do(h, http.MethodGet, path, "")
		var out struct {
			Payments  []struct{ ID string } `json:"payments"`
			NextAfter string                `json:"next_after"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || rec.Code != 200 {
			t.Fatalf("list: %d %s", rec.Code, rec.Body)
		}
		for _, p := range out.Payments {
			if seen[p.ID] {
				t.Fatalf("duplicate %s", p.ID)
			}
			seen[p.ID] = true
		}
		if out.NextAfter == "" {
			break
		}
		after = out.NextAfter
	}
	if len(seen) != 5 {
		t.Fatalf("saw %d payments", len(seen))
	}
}

func TestListAndReconcileRejectBadParams(t *testing.T) {
	h := listReconSetup(nil)
	for _, q := range []string{"limit=0", "limit=201", "limit=x", "after=nope"} {
		for _, route := range []string{"/payments?", "/reconciliation?"} {
			if rec := do(h, http.MethodGet, route+q, ""); rec.Code != 400 {
				t.Errorf("%s%s = %d", route, q, rec.Code)
			}
		}
	}
}

func TestReconcileMatchesAndDetectsMissingLedger(t *testing.T) {
	h := listReconSetup(nil)
	seed(t, h, 3)
	var r query.ReconcileReport
	rec := do(h, http.MethodGet, "/reconciliation", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || r.Checked != 3 || r.Matched != 3 || r.Discrepancies != 0 {
		t.Fatalf("matched report: %d %s", rec.Code, rec.Body)
	}
	// A different ledger knows none of the posted transactions.
	h = listReconSetup(downstream.NewSimulatedLedger())
	seed(t, h, 2)
	r = query.ReconcileReport{}
	rec = do(h, http.MethodGet, "/reconciliation", "")
	if err := json.Unmarshal(rec.Body.Bytes(), &r); err != nil || r.Checked != 2 || r.Discrepancies != 2 {
		t.Fatalf("missing ledger report: %d %s", rec.Code, rec.Body)
	}
	for _, it := range r.Items {
		if it.Status != query.ReconMissingLedger {
			t.Fatalf("status %s", it.Status)
		}
	}
}
