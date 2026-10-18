package domain

import (
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

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
	ID             PaymentID
	IdempotencyKey string
	SourceAccount  string
	DestAccount    string
	Amount         decimal.Decimal
	Currency       string
	Status         PaymentStatus
	LedgerTxID     *string
	RefundedAmount decimal.Decimal
}
