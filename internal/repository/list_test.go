package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
)

type listStore interface {
	PaymentRepository
	PaymentLister
}

func TestMemoryListContract(t *testing.T) { listContract(t, NewInMemoryPayment()) }

func listContract(t *testing.T, repo listStore) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < 5; i++ {
		p := domain.Payment{ID: uuid.New(), IdempotencyKey: uuid.NewString(), SourceAccount: "s", DestAccount: "d", Amount: decimal.NewFromInt(1), Currency: "USD", Status: domain.StatusPending}
		if err := repo.Save(ctx, p); err != nil {
			t.Fatal(err)
		}
	}
	seen := map[uuid.UUID]bool{}
	after, prev := uuid.Nil, ""
	for pages := 0; pages < 10; pages++ {
		page, err := repo.List(ctx, after, 2)
		if err != nil {
			t.Fatal(err)
		}
		if len(page) == 0 {
			break
		}
		if len(page) > 2 {
			t.Fatalf("page size %d", len(page))
		}
		for _, p := range page {
			if p.ID.String() <= prev {
				t.Fatalf("not ascending: %s after %s", p.ID, prev)
			}
			prev = p.ID.String()
			seen[p.ID] = true
		}
		after = page[len(page)-1].ID
	}
	if len(seen) < 5 {
		t.Fatalf("saw %d payments", len(seen))
	}
}
