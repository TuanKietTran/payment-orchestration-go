package command

import (
	"context"
	"errors"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/shopspring/decimal"
)

type refundCompletionFault struct {
	repository.RefundRepository
	fail bool
}

func (r *refundCompletionFault) CompleteRefund(ctx context.Context, id domain.PaymentID, ref string) (*domain.Refund, error) {
	if r.fail {
		r.fail = false
		return nil, errors.New("injected refund completion failure")
	}
	return r.RefundRepository.CompleteRefund(ctx, id, ref)
}

type reverseLedger struct {
	*ambiguousLedger
	source, dest string
	amount       decimal.Decimal
}

func (l *reverseLedger) Post(ctx context.Context, key, source, dest string, amount decimal.Decimal, currency string) (string, error) {
	l.source, l.dest, l.amount = source, dest, amount
	return l.ambiguousLedger.Post(ctx, key, source, dest, amount, currency)
}
func TestRefundRecoversReversePostingWithoutDoubleCounting(t *testing.T) {
	for _, fault := range []string{"response lost", "completion failed"} {
		t.Run(fault, func(t *testing.T) {
			ctx := context.Background()
			memory := repository.NewInMemoryPayment()
			ledger := &reverseLedger{ambiguousLedger: &ambiguousLedger{SimulatedLedger: downstream.NewSimulatedLedger()}}
			id, err := NewCreatePaymentHandler(memory).Handle(ctx, CreatePaymentCommand{IdempotencyKey: "payment", SourceAccount: "customer", DestAccount: "merchant", Amount: decimal.NewFromInt(10), Currency: "USD"})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewProcessPaymentHandler(memory, downstream.SimulatedFraud{}, ledger).Handle(ctx, id); err != nil {
				t.Fatal(err)
			}
			repo := &refundCompletionFault{RefundRepository: memory, fail: fault == "completion failed"}
			ledger.fail = fault == "response lost"
			h := NewRefundPaymentHandler(repo, ledger)
			if _, err := h.Handle(ctx, id, "first-refund", decimal.NewFromInt(3)); err == nil {
				t.Fatal("fault not observed")
			}
			p, err := memory.GetByID(ctx, id)
			if err != nil || p.Status != domain.StatusSettled || !p.RefundedAmount.IsZero() {
				t.Fatalf("premature refund %+v %v", p, err)
			}
			if ledger.source != "merchant" || ledger.dest != "customer" || !ledger.amount.Equal(decimal.NewFromInt(3)) {
				t.Fatalf("not reversed %+v", ledger)
			}
			restarted := NewRefundPaymentHandler(memory, ledger)
			if err := restarted.Recover(ctx, 100); err != nil {
				t.Fatal(err)
			}
			r, err := restarted.Handle(ctx, id, "first-refund", decimal.RequireFromString("3.00"))
			if err != nil || r.LedgerTxID == nil || r.Status != domain.StatusSettled {
				t.Fatalf("refund=%+v err=%v", r, err)
			}
			if len(ledger.Transactions) != 2 || ledger.Transactions["refund:"+r.ID.String()] != *r.LedgerTxID {
				t.Fatalf("duplicate or missing posting: %v", ledger.Transactions)
			}
			p, err = memory.GetByID(ctx, id)
			if err != nil || p.Status != domain.StatusPartiallyRefunded || !p.RefundedAmount.Equal(decimal.NewFromInt(3)) {
				t.Fatalf("payment=%+v err=%v", p, err)
			}
			if _, err := restarted.Handle(ctx, id, "first-refund", decimal.NewFromInt(4)); !errors.Is(err, domain.ErrRefundConflict) {
				t.Fatal(err)
			}
			if _, err := restarted.Handle(ctx, id, "over-refund", decimal.NewFromInt(8)); !errors.Is(err, domain.ErrInvalidRefund) {
				t.Fatal(err)
			}
			if _, err := restarted.Handle(ctx, id, "final-refund", decimal.NewFromInt(7)); err != nil {
				t.Fatal(err)
			}
			p, err = memory.GetByID(ctx, id)
			if err != nil || p.Status != domain.StatusRefunded || !p.RefundedAmount.Equal(p.Amount) || len(ledger.Transactions) != 3 {
				t.Fatalf("payment=%+v ledger=%v err=%v", p, ledger.Transactions, err)
			}
		})
	}
}
