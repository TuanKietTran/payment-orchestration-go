package command

import (
	"context"

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

type CreatePaymentHandler struct {
	repo repository.PaymentRepository
}

func NewCreatePaymentHandler(repo repository.PaymentRepository) *CreatePaymentHandler {
	return &CreatePaymentHandler{repo: repo}
}

func (h *CreatePaymentHandler) Handle(ctx context.Context, cmd CreatePaymentCommand) (domain.PaymentID, error) {
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
		return uuid.Nil, err
	}
	return p.ID, nil
}
