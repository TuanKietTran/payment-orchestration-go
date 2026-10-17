package repository

import (
	"context"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
)

type PaymentRepository interface {
	Save(ctx context.Context, p domain.Payment) error
	GetByID(ctx context.Context, id domain.PaymentID) (*domain.Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error)
	UpdateStatus(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error
}
