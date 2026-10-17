package command

import (
	"context"
	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
	"testing"
)

type countLedger struct{ n int }

func (l *countLedger) Post(context.Context, string, string, string, decimal.Decimal, string) (string, error) {
	l.n++
	return uuid.NewString(), nil
}
func TestProcessAllowReviewAndDeny(t *testing.T) {
	for _, tc := range []struct {
		name   string
		amount int64
		want   domain.PaymentStatus
		posts  int
	}{{"allow", 100, domain.StatusSettled, 1}, {"review", 6000, domain.StatusScreened, 0}, {"deny", 20000, domain.StatusFailed, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			repo := repository.NewInMemoryPayment()
			create := NewCreatePaymentHandler(repo)
			id, err := create.Handle(context.Background(), CreatePaymentCommand{IdempotencyKey: "stable-" + tc.name, SourceAccount: "source", DestAccount: "dest", Amount: decimal.NewFromInt(tc.amount), Currency: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			ledger := &countLedger{}
			h := NewProcessPaymentHandler(repo, downstream.SimulatedFraud{}, ledger)
			p, err := h.Handle(context.Background(), id)
			if err != nil {
				t.Fatal(err)
			}
			if p.Status != tc.want || ledger.n != tc.posts {
				t.Fatalf("status=%s posts=%d", p.Status, ledger.n)
			}
			if _, err = h.Handle(context.Background(), id); err != nil {
				t.Fatal(err)
			}
			if ledger.n != tc.posts {
				t.Fatalf("reprocess duplicated posting: %d", ledger.n)
			}
		})
	}
}
