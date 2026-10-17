package command

import (
	"context"
	"strings"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/query"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestCreateAndGetPayment(t *testing.T) {
	repo := repository.NewInMemoryPayment()
	create := NewCreatePaymentHandler(repo)
	get := query.NewGetPaymentHandler(repo)

	id, err := create.Handle(context.Background(), CreatePaymentCommand{
		IdempotencyKey: "pay-1",
		SourceAccount:  "acct-src",
		DestAccount:    "acct-dst",
		Amount:         decimal.RequireFromString("19.99"),
		Currency:       "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if id == uuid.Nil {
		t.Fatal("expected payment id")
	}

	got, err := get.Handle(context.Background(), query.GetPaymentQuery{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	if got.IdempotencyKey != "pay-1" || got.SourceAccount != "acct-src" || got.DestAccount != "acct-dst" {
		t.Fatalf("unexpected payment: %+v", got)
	}
	if !got.Amount.Equal(decimal.RequireFromString("19.99")) || got.Currency != "USD" {
		t.Fatalf("amount/currency = %s %s", got.Amount, got.Currency)
	}
	if got.Status != domain.StatusPending {
		t.Fatalf("status = %s, want PENDING", got.Status)
	}
}

func TestCreatePayment_IdempotencyKey(t *testing.T) {
	repo := repository.NewInMemoryPayment()
	create := NewCreatePaymentHandler(repo)
	cmd := CreatePaymentCommand{
		IdempotencyKey: "same-pay",
		SourceAccount:  "a",
		DestAccount:    "b",
		Amount:         decimal.NewFromInt(3),
		Currency:       "EUR",
	}
	id, err := create.Handle(context.Background(), cmd)
	if err != nil {
		t.Fatal(err)
	}
	_, err = create.Handle(context.Background(), cmd)
	if err == nil || !strings.Contains(err.Error(), "idempotency key already used") || !strings.Contains(err.Error(), id.String()) {
		t.Fatalf("err = %v", err)
	}
}

func TestGetPayment_NotFound(t *testing.T) {
	repo := repository.NewInMemoryPayment()
	_, err := query.NewGetPaymentHandler(repo).Handle(context.Background(), query.GetPaymentQuery{ID: uuid.New()})
	if err == nil || !strings.Contains(err.Error(), "payment not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestUpdateStatusThenGet(t *testing.T) {
	repo := repository.NewInMemoryPayment()
	create := NewCreatePaymentHandler(repo)
	get := query.NewGetPaymentHandler(repo)

	id, err := create.Handle(context.Background(), CreatePaymentCommand{
		IdempotencyKey: "pay-status",
		SourceAccount:  "a",
		DestAccount:    "b",
		Amount:         decimal.NewFromInt(8),
		Currency:       "USD",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateStatus(context.Background(), id, domain.StatusSucceeded); err != nil {
		t.Fatal(err)
	}
	got, err := get.Handle(context.Background(), query.GetPaymentQuery{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusSucceeded {
		t.Fatalf("status = %s", got.Status)
	}

	err = repo.UpdateStatus(context.Background(), uuid.New(), domain.StatusFailed)
	if err == nil || !strings.Contains(err.Error(), "payment not found") {
		t.Fatalf("err = %v", err)
	}
}

type errRepo struct{ err error }

func (e errRepo) Save(context.Context, domain.Payment) error { return e.err }
func (e errRepo) GetByID(context.Context, domain.PaymentID) (*domain.Payment, error) {
	return nil, e.err
}
func (e errRepo) UpdateStatus(context.Context, domain.PaymentID, domain.PaymentStatus) error {
	return e.err
}

func TestHandlers_PropagateRepositoryErrors(t *testing.T) {
	repo := errRepo{err: context.Canceled}
	_, err := NewCreatePaymentHandler(repo).Handle(context.Background(), CreatePaymentCommand{
		IdempotencyKey: "x",
		Amount:         decimal.NewFromInt(1),
		Currency:       "USD",
	})
	if err != context.Canceled {
		t.Fatalf("create err = %v", err)
	}
	_, err = query.NewGetPaymentHandler(repo).Handle(context.Background(), query.GetPaymentQuery{ID: uuid.New()})
	if err != context.Canceled {
		t.Fatalf("get err = %v", err)
	}
}
