package repository

import (
	"context"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
)

type PaymentStore interface {
	ProcessingRepository
	RefundRepository
}

// ProcessingRepository atomically records a fraud decision and, for ALLOW, a
// recoverable ledger intent. Completion stores the posting reference and removes
// the intent in the same transaction. Downstream posting must be idempotent.
type ProcessingRepository interface {
	PaymentRepository
	StageDecision(context.Context, domain.PaymentID, domain.PaymentStatus) error
	CompletePosting(context.Context, domain.PaymentID, string) error
	AdvancePSP(context.Context, domain.PaymentID, domain.PaymentStatus, domain.PaymentStatus, string) error
	DeclinePSP(context.Context, domain.PaymentID) error
	PendingPostings(context.Context, int) ([]domain.PaymentID, error)
}

type PaymentRepository interface {
	Save(ctx context.Context, p domain.Payment) error
	GetByID(ctx context.Context, id domain.PaymentID) (*domain.Payment, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Payment, error)
	UpdateStatus(ctx context.Context, id domain.PaymentID, status domain.PaymentStatus) error
}
