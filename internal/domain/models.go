package domain

import (
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrPaymentNotFound = errors.New("payment not found")

type PaymentID = uuid.UUID

type PaymentStatus string

const (
	StatusPending    PaymentStatus = "PENDING"
	StatusProcessing PaymentStatus = "PROCESSING"
	StatusSucceeded  PaymentStatus = "SUCCEEDED"
	StatusFailed     PaymentStatus = "FAILED"
	StatusRefunded   PaymentStatus = "REFUNDED"
)

type Payment struct {
	ID              PaymentID
	IdempotencyKey  string
	SourceAccount   string
	DestAccount     string
	Amount          decimal.Decimal
	Currency        string
	Status          PaymentStatus
	LedgerTxID      *string
	RefundedAmount  decimal.Decimal
	FraudDecision   string
	AuthorizationID string
	CaptureID       string
}
