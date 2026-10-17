package command

import (
	"context"
	"errors"
	"fmt"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type CreatePaymentCommand struct {
	IdempotencyKey string
	SourceAccount  string
	DestAccount    string
	Amount         decimal.Decimal
	Currency       string
}

var ErrIdempotencyConflict = errors.New("idempotency key reused with different payload")

type CreatePaymentHandler struct {
	repo repository.PaymentRepository
}

func NewCreatePaymentHandler(repo repository.PaymentRepository) *CreatePaymentHandler {
	return &CreatePaymentHandler{repo: repo}
}

func (h *CreatePaymentHandler) Handle(ctx context.Context, cmd CreatePaymentCommand) (domain.PaymentID, error) {
	if cmd.IdempotencyKey == "" {
		return uuid.Nil, fmt.Errorf("idempotency key is required")
	}
	if err := domain.ValidateAmountCurrency(cmd.Amount, cmd.Currency); err != nil {
		return uuid.Nil, err
	}
	p := domain.Payment{
		ID:             uuid.New(),
		IdempotencyKey: cmd.IdempotencyKey,
		SourceAccount:  cmd.SourceAccount,
		DestAccount:    cmd.DestAccount,
		Amount:         cmd.Amount,
		Currency:       cmd.Currency,
		Status:         domain.StatusPending,
	}
	if err := h.repo.Save(ctx, p); err != nil {
		existing, lookupErr := h.repo.GetByIdempotencyKey(ctx, cmd.IdempotencyKey)
		if lookupErr != nil {
			return uuid.Nil, err
		}
		if existing.SourceAccount != p.SourceAccount || existing.DestAccount != p.DestAccount || !existing.Amount.Equal(p.Amount) || existing.Currency != p.Currency {
			return uuid.Nil, ErrIdempotencyConflict
		}
		return existing.ID, nil
	}
	return p.ID, nil
}
