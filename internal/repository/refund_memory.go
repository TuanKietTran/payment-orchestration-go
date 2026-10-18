package repository

import (
	"context"
	"fmt"
	"sort"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func cloneRefund(r domain.Refund) domain.Refund {
	if r.LedgerTxID != nil {
		ref := *r.LedgerTxID
		r.LedgerTxID = &ref
	}
	return r
}
func sameRefund(r domain.Refund, id domain.PaymentID, amount decimal.Decimal) error {
	if r.PaymentID != id || !r.Amount.Equal(amount) {
		return domain.ErrRefundConflict
	}
	return nil
}
func validRefundRequest(key string, amount decimal.Decimal) error {
	if key == "" || !amount.Equal(amount.Round(2)) || amount.Sign() <= 0 {
		return domain.ErrInvalidRefund
	}
	return nil
}
func (r *InMemoryPayment) ReserveRefund(ctx context.Context, id domain.PaymentID, key string, amount decimal.Decimal) (*domain.Refund, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validRefundRequest(key, amount); err != nil {
		return nil, err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if rid, ok := r.refundKeys[key]; ok {
		refund := cloneRefund(r.refunds[rid])
		if err := sameRefund(refund, id, amount); err != nil {
			return nil, err
		}
		return &refund, nil
	}
	p, ok := r.payments[id]
	if !ok {
		return nil, fmt.Errorf("payment not found")
	}
	if !domain.CanRefund(p.Status) {
		return nil, domain.ErrInvalidRefund
	}
	total := decimal.Zero
	for _, refund := range r.refunds {
		if refund.PaymentID == id {
			total = total.Add(refund.Amount)
		}
	}
	if err := domain.ValidateRefund(p.Amount, total, amount); err != nil {
		return nil, err
	}
	refund := domain.Refund{ID: uuid.New(), PaymentID: id, IdempotencyKey: key, Amount: amount, Currency: p.Currency, Status: domain.StatusPending}
	r.refunds[refund.ID] = refund
	r.refundKeys[key] = refund.ID
	return &refund, nil
}
func (r *InMemoryPayment) CompleteRefund(ctx context.Context, id uuid.UUID, ref string) (*domain.Refund, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, err := uuid.Parse(ref); err != nil {
		return nil, fmt.Errorf("ledger tx id: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	refund, ok := r.refunds[id]
	if !ok {
		return nil, fmt.Errorf("refund not found")
	}
	if refund.Status == domain.StatusSettled {
		if refund.LedgerTxID == nil || *refund.LedgerTxID != ref {
			return nil, domain.ErrRefundConflict
		}
		copy := cloneRefund(refund)
		return &copy, nil
	}
	p := r.payments[refund.PaymentID]
	if err := domain.ValidateRefund(p.Amount, p.RefundedAmount, refund.Amount); err != nil {
		return nil, err
	}
	p.RefundedAmount = p.RefundedAmount.Add(refund.Amount)
	p.Status = domain.RefundStatus(p.RefundedAmount, p.Amount)
	refund.Status, refund.LedgerTxID = domain.StatusSettled, &ref
	r.payments[p.ID] = p
	r.refunds[id] = refund
	copy := cloneRefund(refund)
	return &copy, nil
}
func (r *InMemoryPayment) PendingRefunds(ctx context.Context, limit int) ([]domain.Refund, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 1000 {
		return nil, fmt.Errorf("limit must be between 1 and 1000")
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	pending := make([]domain.Refund, 0)
	for _, refund := range r.refunds {
		if refund.Status == domain.StatusPending {
			pending = append(pending, cloneRefund(refund))
		}
	}
	sort.Slice(pending, func(i, j int) bool { return pending[i].ID.String() < pending[j].ID.String() })
	if len(pending) > limit {
		pending = pending[:limit]
	}
	return pending, nil
}
