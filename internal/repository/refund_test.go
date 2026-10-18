package repository

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestMemoryRefundContract(t *testing.T) { refundContract(t, NewInMemoryPayment()) }
func refundContract(t *testing.T, repo RefundRepository) {
	t.Helper()
	ctx := context.Background()
	p := domain.Payment{ID: uuid.New(), IdempotencyKey: uuid.NewString(), Amount: decimal.NewFromInt(10), Currency: "USD", SourceAccount: "source", DestAccount: "dest", Status: domain.StatusSettled}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	key := uuid.NewString()
	for _, amount := range []string{"0", "-1", "0.001", "10.01"} {
		if _, err := repo.ReserveRefund(ctx, p.ID, key, decimal.RequireFromString(amount)); !errors.Is(err, domain.ErrInvalidRefund) {
			t.Fatalf("amount=%s err=%v", amount, err)
		}
	}
	if _, err := repo.ReserveRefund(ctx, p.ID, "", decimal.NewFromInt(1)); !errors.Is(err, domain.ErrInvalidRefund) {
		t.Fatal(err)
	}
	if _, err := repo.ReserveRefund(ctx, uuid.New(), key, decimal.NewFromInt(1)); err == nil {
		t.Fatal("reserved missing payment")
	}
	first, err := repo.ReserveRefund(ctx, p.ID, key, decimal.NewFromInt(3))
	if err != nil {
		t.Fatal(err)
	}
	replay, err := repo.ReserveRefund(ctx, p.ID, key, decimal.RequireFromString("3.00"))
	if err != nil || replay.ID != first.ID {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	if _, err := repo.ReserveRefund(ctx, p.ID, key, decimal.NewFromInt(4)); !errors.Is(err, domain.ErrRefundConflict) {
		t.Fatal(err)
	}
	other := p
	other.ID, other.IdempotencyKey = uuid.New(), uuid.NewString()
	other.Status = domain.StatusPending
	if err := repo.Save(ctx, other); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.ReserveRefund(ctx, other.ID, key, decimal.NewFromInt(3)); !errors.Is(err, domain.ErrRefundConflict) {
		t.Fatal(err)
	}
	if _, err := repo.ReserveRefund(ctx, other.ID, uuid.NewString(), decimal.NewFromInt(1)); !errors.Is(err, domain.ErrInvalidRefund) {
		t.Fatal(err)
	}
	if _, err := repo.CompleteRefund(ctx, first.ID, "invalid"); err == nil {
		t.Fatal("accepted invalid reference")
	}
	if _, err := repo.CompleteRefund(ctx, uuid.New(), uuid.NewString()); err == nil {
		t.Fatal("completed missing refund")
	}
	ref := uuid.NewString()
	settled, err := repo.CompleteRefund(ctx, first.ID, ref)
	if err != nil || settled.LedgerTxID == nil || *settled.LedgerTxID != ref {
		t.Fatalf("refund=%+v err=%v", settled, err)
	}
	*settled.LedgerTxID = "caller mutation"
	replay, err = repo.CompleteRefund(ctx, first.ID, ref)
	if err != nil || *replay.LedgerTxID != ref {
		t.Fatalf("aliasing %+v %v", replay, err)
	}
	if _, err := repo.CompleteRefund(ctx, first.ID, uuid.NewString()); !errors.Is(err, domain.ErrRefundConflict) {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil || got.Status != domain.StatusPartiallyRefunded || !got.RefundedAmount.Equal(decimal.NewFromInt(3)) {
		t.Fatalf("payment=%+v err=%v", got, err)
	}
	// Reserve concurrently: pending reservations count against the cap as well.
	var wg sync.WaitGroup
	var mu sync.Mutex
	refunds := make([]domain.Refund, 0)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r, err := repo.ReserveRefund(ctx, p.ID, uuid.NewString(), decimal.NewFromInt(1))
			if err != nil {
				if !errors.Is(err, domain.ErrInvalidRefund) {
					t.Error(err)
				}
				return
			}
			mu.Lock()
			refunds = append(refunds, *r)
			mu.Unlock()
		}()
	}
	wg.Wait()
	if len(refunds) != 7 {
		t.Fatalf("reserved %d, want 7", len(refunds))
	}
	pending, err := repo.PendingRefunds(ctx, 1)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	for _, refund := range refunds {
		wg.Add(1)
		go func(r domain.Refund) {
			defer wg.Done()
			ref := uuid.NewString()
			if _, err := repo.CompleteRefund(ctx, r.ID, ref); err != nil {
				t.Error(err)
			}
			if _, err := repo.CompleteRefund(ctx, r.ID, ref); err != nil {
				t.Error(err)
			}
		}(refund)
	}
	wg.Wait()
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil || got.Status != domain.StatusRefunded || !got.RefundedAmount.Equal(p.Amount) {
		t.Fatalf("payment=%+v err=%v", got, err)
	}
	if _, err := repo.ReserveRefund(ctx, p.ID, uuid.NewString(), decimal.NewFromInt(1)); !errors.Is(err, domain.ErrInvalidRefund) {
		t.Fatal(err)
	}
	replay, err = repo.ReserveRefund(ctx, p.ID, key, decimal.NewFromInt(3))
	if err != nil || replay.Status != domain.StatusSettled {
		t.Fatalf("replay=%+v err=%v", replay, err)
	}
	pending, err = repo.PendingRefunds(ctx, 100)
	if err != nil || len(pending) != 0 {
		t.Fatalf("pending=%v err=%v", pending, err)
	}
	for _, limit := range []int{0, 1001} {
		if _, err := repo.PendingRefunds(ctx, limit); err == nil {
			t.Fatal("accepted invalid limit")
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := repo.PendingRefunds(canceled, 1); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := repo.ReserveRefund(canceled, p.ID, "cancel", decimal.NewFromInt(1)); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := repo.CompleteRefund(canceled, first.ID, ref); err == nil {
		t.Fatal("ignored cancellation")
	}
}
