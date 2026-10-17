package query

import (
	"context"

	"github.com/example/payment-orchestration/internal/domain"
	"github.com/example/payment-orchestration/internal/repository"
)

type GetPaymentQuery struct {
	ID domain.PaymentID
}

type GetPaymentHandler struct {
	repo repository.PaymentRepository
}

func NewGetPaymentHandler(repo repository.PaymentRepository) *GetPaymentHandler {
	return &GetPaymentHandler{repo: repo}
}

func (h *GetPaymentHandler) Handle(ctx context.Context, q GetPaymentQuery) (*domain.Payment, error) {
	return h.repo.GetByID(ctx, q.ID)
}
