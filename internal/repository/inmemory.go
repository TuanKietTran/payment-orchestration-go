package repository

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
)

type InMemoryPayment struct {
	mu          sync.RWMutex
	payments    map[domain.PaymentID]domain.Payment
	idempotency map[string]domain.PaymentID
	outbox      map[domain.PaymentID]bool
	refunds     map[uuid.UUID]domain.Refund
	refundKeys  map[string]uuid.UUID
}

func NewInMemoryPayment() *InMemoryPayment {
	return &InMemoryPayment{
		payments:    make(map[domain.PaymentID]domain.Payment),
		idempotency: make(map[string]domain.PaymentID),
		outbox:      make(map[domain.PaymentID]bool),
		refunds:     make(map[uuid.UUID]domain.Refund),
		refundKeys:  make(map[string]uuid.UUID),
	}
}

func (r *InMemoryPayment) Save(ctx context.Context, p domain.Payment) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, ok := r.idempotency[p.IdempotencyKey]; ok {
		return fmt.Errorf("idempotency key already used: %s", existing)
	}
	r.payments[p.ID] = clonePayment(p)
	r.idempotency[p.IdempotencyKey] = p.ID
	return nil
}

func (r *InMemoryPayment) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	id, ok := r.idempotency[key]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}
	p := clonePayment(r.payments[id])
	return &p, nil
}

func (r *InMemoryPayment) GetByID(ctx context.Context, id domain.PaymentID) (*domain.Payment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}
	p = clonePayment(p)
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

func clonePayment(p domain.Payment) domain.Payment {
	if p.LedgerTxID != nil {
		ref := *p.LedgerTxID
		p.LedgerTxID = &ref
	}
	return p
}

func (r *InMemoryPayment) StageDecision(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !validDecisionStatus(status) {
		return fmt.Errorf("invalid decision status %q", status)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("payment not found")
	}
	if p.Status != domain.StatusPending && p.Status != domain.StatusCreated {
		return nil
	}
	target, decision := status, "REVIEW"
	if status == domain.StatusProcessing {
		target, decision = domain.StatusScreened, "ALLOW"
	} else if status == domain.StatusFailed {
		decision = "DENY"
	}
	updated, err := domain.Transition(p, target)
	if err != nil {
		return err
	}
	updated.FraudDecision = decision
	if status == domain.StatusProcessing {
		r.outbox[id] = true
	}
	r.payments[id] = updated
	return nil
}

func validDecisionStatus(status domain.PaymentStatus) bool {
	return status == domain.StatusProcessing || status == domain.StatusScreened || status == domain.StatusFailed
}

func (r *InMemoryPayment) CompletePosting(ctx context.Context, id domain.PaymentID, tx string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, err := uuid.Parse(tx); err != nil {
		return fmt.Errorf("ledger tx id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("payment not found")
	}
	if domain.CanRefund(p.Status) && p.LedgerTxID != nil && *p.LedgerTxID == tx {
		return nil
	}
	if !r.outbox[id] || p.Status != domain.StatusCaptured {
		return fmt.Errorf("posting intent missing or conflicting")
	}
	updated, err := domain.Transition(p, domain.StatusSettled)
	if err != nil {
		return err
	}
	updated.LedgerTxID = &tx
	r.payments[id] = updated
	delete(r.outbox, id)
	return nil
}

func (r *InMemoryPayment) PendingPostings(ctx context.Context, limit int) ([]domain.PaymentID, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]domain.PaymentID, 0, len(r.outbox))
	for id := range r.outbox {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}
