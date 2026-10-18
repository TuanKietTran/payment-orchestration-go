package repository

import (
	"context"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
)

func validPSPStep(from, to domain.PaymentStatus, ref string) error {
	if _, err := uuid.Parse(ref); err != nil {
		return fmt.Errorf("PSP reference: %w", err)
	}
	if (from != domain.StatusScreened || to != domain.StatusAuthorized) && (from != domain.StatusAuthorized || to != domain.StatusCaptured) {
		return domain.ErrIllegalTransition
	}
	return nil
}
func applyPSPStep(p domain.Payment, from, to domain.PaymentStatus, ref string) (domain.Payment, error) {
	if err := validPSPStep(from, to, ref); err != nil {
		return p, err
	}
	existing := p.AuthorizationID
	if to == domain.StatusCaptured {
		existing = p.CaptureID
	}
	if existing != "" {
		if existing != ref {
			return p, fmt.Errorf("conflicting PSP reference")
		}
		return p, nil
	}
	if p.FraudDecision != "ALLOW" {
		return p, domain.ErrIllegalTransition
	}
	updated, err := domain.Transition(p, to)
	if err != nil || p.Status != from {
		return p, domain.ErrIllegalTransition
	}
	if to == domain.StatusAuthorized {
		updated.AuthorizationID = ref
	} else {
		updated.CaptureID = ref
	}
	return updated, nil
}
func (r *InMemoryPayment) AdvancePSP(ctx context.Context, id domain.PaymentID, from, to domain.PaymentStatus, ref string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("payment not found")
	}
	updated, err := applyPSPStep(p, from, to, ref)
	if err != nil {
		return err
	}
	if updated.Status == p.Status {
		return nil
	}
	if !r.outbox[id] {
		return domain.ErrIllegalTransition
	}
	r.payments[id] = updated
	return nil
}
func (r *InMemoryPayment) DeclinePSP(ctx context.Context, id domain.PaymentID) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.payments[id]
	if !ok {
		return fmt.Errorf("payment not found")
	}
	if p.Status == domain.StatusFailed {
		return nil
	}
	if !r.outbox[id] || (p.Status != domain.StatusScreened && p.Status != domain.StatusAuthorized) {
		return domain.ErrIllegalTransition
	}
	updated, err := domain.Transition(p, domain.StatusFailed)
	if err != nil {
		return err
	}
	r.payments[id] = updated
	delete(r.outbox, id)
	return nil
}
