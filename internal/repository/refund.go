package repository

import (
	"context"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type RefundRepository interface {
	PaymentRepository
	ReserveRefund(context.Context, domain.PaymentID, string, decimal.Decimal) (*domain.Refund, error)
	CompleteRefund(context.Context, uuid.UUID, string) (*domain.Refund, error)
	PendingRefunds(context.Context, int) ([]domain.Refund, error)
}
