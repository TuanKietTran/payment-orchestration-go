package api

import (
	"context"
	"encoding/json"
	"github.com/TuanKietTran/payment-orchestration-go/internal/command"
	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/shopspring/decimal"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProcessOverHTTPReviewParksWithoutLedgerPosting(t *testing.T) {
	r := repository.NewInMemoryPayment()
	create := command.NewCreatePaymentHandler(r)
	id, err := create.Handle(context.Background(), command.CreatePaymentCommand{IdempotencyKey: "review", SourceAccount: "account", DestAccount: "dest", Amount: decimal.NewFromInt(6000), Currency: "USD"})
	if err != nil {
		t.Fatal(err)
	}
	ledger := downstream.NewSimulatedLedger()
	h := NewHandlers(create, query.NewGetPaymentHandler(r), command.NewProcessPaymentHandler(r, downstream.SimulatedFraud{}, ledger))
	s := httptest.NewServer(h.Routes())
	defer s.Close()
	req, _ := http.NewRequest(http.MethodPost, s.URL+"/payments/"+id.String()+"/process", strings.NewReader("{}"))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var p domain.Payment
	if err = json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatal(err)
	}
	if p.Status != domain.StatusScreened || len(ledger.Transactions) != 0 {
		t.Fatalf("status=%s ledger=%v", p.Status, ledger.Transactions)
	}
}
