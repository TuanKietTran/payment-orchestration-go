package main

import (
	"context"
	"fmt"
	"log"

	"github.com/example/payment-orchestration/internal/command"
	"github.com/example/payment-orchestration/internal/domain"
	"github.com/example/payment-orchestration/internal/query"
	"github.com/example/payment-orchestration/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type memRepo struct {
	store map[uuid.UUID]domain.Payment
}

func (m *memRepo) Save(ctx context.Context, p domain.Payment) error {
	if m.store == nil {
		m.store = make(map[uuid.UUID]domain.Payment)
	}
	m.store[p.ID] = p
	return nil
}
func (m *memRepo) GetByID(ctx context.Context, id domain.PaymentID) (*domain.Payment, error) {
	p, ok := m.store[id]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return &p, nil
}
func (m *memRepo) UpdateStatus(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error {
	return nil
}

func main() {
	repo := &memRepo{}
	createH := command.NewCreatePaymentHandler(repo)
	getH := query.NewGetPaymentHandler(repo)

	id, err := createH.Handle(context.Background(), command.CreatePaymentCommand{
		IdempotencyKey: "pay-1",
		SourceAccount:  "acc-1",
		DestAccount:    "acc-2",
		Amount:         decimal.NewFromInt(250),
		Currency:       "USD",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("Created payment:", id)

	p, err := getH.Handle(context.Background(), query.GetPaymentQuery{ID: id})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Payment status: %s amount: %s\n", p.Status, p.Amount)
}
