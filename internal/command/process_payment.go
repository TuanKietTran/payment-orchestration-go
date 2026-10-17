package command

import (
	"context"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
)

type ProcessPaymentHandler struct {
	repo   repository.PaymentRepository
	fraud  downstream.Fraud
	ledger downstream.Ledger
}

func NewProcessPaymentHandler(r repository.PaymentRepository, f downstream.Fraud, l downstream.Ledger) *ProcessPaymentHandler {
	return &ProcessPaymentHandler{r, f, l}
}
func (h *ProcessPaymentHandler) Handle(ctx context.Context, id domain.PaymentID) (*domain.Payment, error) {
	p, err := h.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p.Status == domain.StatusSettled || p.Status == domain.StatusFailed || p.Status == domain.StatusScreened {
		return p, nil
	}
	d, err := h.fraud.Score(ctx, p.IdempotencyKey, p.SourceAccount, p.Amount, p.Currency)
	if err != nil {
		return nil, err
	}
	decision := d.Decision
	if decision == "DENY" {
		decision = "BLOCK"
	}
	if decision == "BLOCK" {
		if err = h.repo.UpdateStatus(ctx, id, domain.StatusFailed); err != nil {
			return nil, err
		}
	} else if decision == "REVIEW" {
		if err = h.repo.UpdateStatus(ctx, id, domain.StatusScreened); err != nil {
			return nil, err
		}
	} else if decision == "ALLOW" {
		tx, err := h.ledger.Post(ctx, "payment:"+p.IdempotencyKey, p.SourceAccount, p.DestAccount, p.Amount, p.Currency)
		if err != nil {
			return nil, err
		}
		if tx == "" {
			return nil, fmt.Errorf("ledger returned empty transaction id")
		}
		if err = h.repo.UpdateStatus(ctx, id, domain.StatusSettled); err != nil {
			return nil, err
		}
	} else {
		return nil, fmt.Errorf("unknown fraud decision %q", decision)
	}
	return h.repo.GetByID(ctx, id)
}
