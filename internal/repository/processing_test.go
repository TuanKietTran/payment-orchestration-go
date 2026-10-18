package repository

import (
	"context"
	"sync"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestMemoryProcessingContract(t *testing.T) { processingContract(t, NewInMemoryPayment()) }

func processingContract(t *testing.T, repo ProcessingRepository) {
	t.Helper()
	ctx := context.Background()
	p := domain.Payment{ID: uuid.New(), IdempotencyKey: uuid.NewString(), Amount: decimal.NewFromInt(10), Currency: "USD", SourceAccount: "source", DestAccount: "dest", Status: domain.StatusPending}
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := repo.StageDecision(ctx, p.ID, domain.StatusSettled); err == nil {
		t.Fatal("accepted invalid decision")
	}
	if err := repo.CompletePosting(ctx, p.ID, uuid.NewString()); err == nil {
		t.Fatal("completed without intent")
	}
	if err := repo.StageDecision(ctx, uuid.New(), domain.StatusProcessing); err == nil {
		t.Fatal("staged missing payment")
	}
	if err := repo.CompletePosting(ctx, uuid.New(), uuid.NewString()); err == nil {
		t.Fatal("completed missing payment")
	}
	if err := repo.StageDecision(ctx, p.ID, domain.StatusProcessing); err != nil {
		t.Fatal(err)
	}
	// A later conflicting fraud decision cannot erase a committed intent.
	if err := repo.StageDecision(ctx, p.ID, domain.StatusFailed); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil || got.Status != domain.StatusScreened || got.FraudDecision != "ALLOW" {
		t.Fatalf("payment=%+v err=%v", got, err)
	}
	ids, err := repo.PendingPostings(ctx, 1)
	if err != nil || len(ids) != 1 || ids[0] != p.ID {
		t.Fatalf("pending=%v err=%v", ids, err)
	}
	for _, limit := range []int{0, 1001} {
		if _, err := repo.PendingPostings(ctx, limit); err == nil {
			t.Fatal("accepted invalid batch limit")
		}
	}
	if err := repo.CompletePosting(ctx, p.ID, "invalid"); err == nil {
		t.Fatal("accepted invalid reference")
	}
	if err := repo.CompletePosting(ctx, p.ID, uuid.NewString()); err == nil {
		t.Fatal("settled without capture evidence")
	}
	advanceCapture(t, repo, p.ID)
	ref := uuid.NewString()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := repo.CompletePosting(ctx, p.ID, ref); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil || got.Status != domain.StatusSettled || got.LedgerTxID == nil || *got.LedgerTxID != ref {
		t.Fatalf("payment=%+v err=%v", got, err)
	}
	*got.LedgerTxID = "mutated caller copy"
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil || *got.LedgerTxID != ref {
		t.Fatalf("aliased reference %+v err=%v", got, err)
	}
	if err := repo.CompletePosting(ctx, p.ID, uuid.NewString()); err == nil {
		t.Fatal("accepted conflicting reference")
	}
	ids, err = repo.PendingPostings(ctx, 10)
	if err != nil || len(ids) != 0 {
		t.Fatalf("pending=%v err=%v", ids, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := repo.StageDecision(canceled, p.ID, domain.StatusProcessing); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err := repo.CompletePosting(canceled, p.ID, ref); err == nil {
		t.Fatal("ignored cancellation")
	}
	if _, err := repo.PendingPostings(canceled, 1); err == nil {
		t.Fatal("ignored cancellation")
	}
	if err := repo.DeclinePSP(ctx, uuid.New()); err == nil {
		t.Fatal("declined missing payment")
	}
	p.ID, p.IdempotencyKey, p.Status = uuid.New(), uuid.NewString(), domain.StatusPending
	if err := repo.Save(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeclinePSP(ctx, p.ID); err == nil {
		t.Fatal("declined without processing intent")
	}
	if err := repo.StageDecision(ctx, p.ID, domain.StatusProcessing); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeclinePSP(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeclinePSP(ctx, p.ID); err != nil {
		t.Fatal(err)
	}
	got, err = repo.GetByID(ctx, p.ID)
	if err != nil || got.Status != domain.StatusFailed {
		t.Fatalf("declined payment=%+v err=%v", got, err)
	}
	for _, status := range []domain.PaymentStatus{domain.StatusFailed, domain.StatusScreened} {
		p.ID, p.IdempotencyKey, p.Status = uuid.New(), uuid.NewString(), domain.StatusPending
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
		if err := repo.StageDecision(ctx, p.ID, status); err != nil {
			t.Fatal(err)
		}
		ids, err := repo.PendingPostings(ctx, 10)
		if err != nil || len(ids) != 0 {
			t.Fatalf("nonallow staged intent: %v %v", ids, err)
		}
	}
}
