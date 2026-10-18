package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/shopspring/decimal"
)

type RefundPaymentHandler struct {
	repo   repository.RefundRepository
	ledger downstream.Ledger
}

func NewRefundPaymentHandler(repo repository.RefundRepository, ledger downstream.Ledger) *RefundPaymentHandler {
	return &RefundPaymentHandler{repo, ledger}
}
func (h *RefundPaymentHandler) Handle(ctx context.Context, id domain.PaymentID, key string, amount decimal.Decimal) (*domain.Refund, error) {
	refund, err := h.repo.ReserveRefund(ctx, id, key, amount)
	if err != nil {
		return nil, err
	}
	return h.dispatch(ctx, *refund)
}
func (h *RefundPaymentHandler) dispatch(ctx context.Context, refund domain.Refund) (*domain.Refund, error) {
	if refund.Status == domain.StatusSettled {
		return &refund, nil
	}
	payment, err := h.repo.GetByID(ctx, refund.PaymentID)
	if err != nil {
		return nil, err
	}
	ref, err := h.ledger.Post(ctx, "refund:"+refund.ID.String(), payment.DestAccount, payment.SourceAccount, refund.Amount, refund.Currency)
	if err != nil {
		return nil, err
	}
	return h.repo.CompleteRefund(ctx, refund.ID, ref)
}
func (h *RefundPaymentHandler) Recover(ctx context.Context, limit int) error {
	pending, err := h.repo.PendingRefunds(ctx, limit)
	if err != nil {
		return err
	}
	var failures []error
	for _, refund := range pending {
		if _, err := h.dispatch(ctx, refund); err != nil {
			failures = append(failures, fmt.Errorf("recover refund %s: %w", refund.ID, err))
		}
	}
	return errors.Join(failures...)
}
