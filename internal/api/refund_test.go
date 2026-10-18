package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/go-chi/chi/v5"
	"github.com/shopspring/decimal"
)

func TestRefundHTTPContractAndReplay(t *testing.T) {
	repo := repository.NewInMemoryPayment()
	ledger := downstream.NewSimulatedLedger()
	create := command.NewCreatePaymentHandler(repo)
	id, err := create.Handle(context.Background(), command.CreatePaymentCommand{IdempotencyKey: "payment", SourceAccount: "customer", DestAccount: "merchant", Amount: decimal.NewFromInt(10), Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	process := command.NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger)
	h := NewHandlers(create, query.NewGetPaymentHandler(repo), process).WithRefund(command.NewRefundPaymentHandler(repo, ledger))
	router := chi.NewRouter()
	router.Mount("/v1", h.Routes())
	captureResponse := httptest.NewRecorder()
	router.ServeHTTP(captureResponse, httptest.NewRequest(http.MethodPost, "/v1/payments/"+id.String()+"/capture", strings.NewReader("{}")))
	if captureResponse.Code != 200 {
		t.Fatalf("capture status=%d body=%s", captureResponse.Code, captureResponse.Body.String())
	}
	var captured domain.Payment
	if err := json.Unmarshal(captureResponse.Body.Bytes(), &captured); err != nil || captured.Status != domain.StatusSettled || captured.AuthorizationID == "" || captured.CaptureID == "" {
		t.Fatalf("payment=%+v err=%v", captured, err)
	}
	path := "/v1/payments/" + id.String() + "/refunds"
	var original domain.Refund
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(`{"amount":"3.00"}`))
		req.Header.Set("Idempotency-Key", "first")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
		}
		var refund domain.Refund
		if err := json.NewDecoder(w.Body).Decode(&refund); err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			original = refund
		} else if original.ID != refund.ID || *original.LedgerTxID != *refund.LedgerTxID {
			t.Fatal("unstable replay")
		}
	}
	if len(ledger.Transactions) != 2 {
		t.Fatalf("postings=%v", ledger.Transactions)
	}
	for _, tc := range []struct {
		path, body, key string
		status          int
	}{
		{path, `{"idempotency_key":"first","amount":"4.00"}`, "", 409},
		{path, `{"idempotency_key":"too-much","amount":"8.00"}`, "", 400},
		{path, `{"amount":"1.00"}`, "", 400},
		{path, `{"idempotency_key":"body","amount":"1.00"}`, "header", 400},
		{path, `{`, "bad", 400},
		{"/v1/payments/invalid/refunds", `{}`, "bad-id", 400},
	} {
		req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
		req.Header.Set("Idempotency-Key", tc.key)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code != tc.status {
			t.Fatalf("body=%s got=%d want=%d", tc.body, w.Code, tc.status)
		}
	}
}
