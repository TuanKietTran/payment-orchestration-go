package repository

import (
	"context"
	"fmt"
	"sync"

	"github.com/example/payment-orchestration/internal/domain"
	"github.com/google/uuid"
)

type InMemoryPayment struct {
	mu          sync.RWMutex
	payments    map[domain.PaymentID]domain.Payment
	idempotency map[string]domain.PaymentID
}

func NewInMemoryPayment() *InMemoryPayment {
	return &InMemoryPayment{
		payments:    make(map[domain.PaymentID]domain.Payment),
		idempotency: make(map[string]domain.PaymentID),
	}
}

func (r *InMemoryPayment) Save(ctx context.Context, p domain.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.idempotency[p.IdempotencyKey]; ok {
		return fmt.Errorf("idempotency key already used: %s", existing)
	}
	r.payments[p.ID] = p
	r.idempotency[p.IdempotencyKey] = p.ID
	return nil
}

func (r *InMemoryPayment) GetByID(ctx context.Context, id domain.PaymentID) (*domain.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}
	return &p, nil
}

func (r *InMemoryPayment) UpdateStatus(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("payment not found")
	}
	p.Status = status
	r.payments[id] = p
	return nil
}
