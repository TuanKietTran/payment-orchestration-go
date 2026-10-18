package domain

import (
	"errors"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

var ErrRefundConflict = errors.New("refund idempotency key reused with different payload")

type Refund struct {
	ID             uuid.UUID
	PaymentID      PaymentID
	IdempotencyKey string
	Amount         decimal.Decimal
	Currency       string
	Status         PaymentStatus
	LedgerTxID     *string
}

func CanRefund(status PaymentStatus) bool {
	return status == StatusSettled || status == StatusPartiallyRefunded || status == StatusRefunded
}

func RefundStatus(total, amount decimal.Decimal) PaymentStatus {
	if total.Equal(amount) {
		return StatusRefunded
	}
	return StatusPartiallyRefunded
}
