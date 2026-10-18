package domain

import (
	"errors"
	"strings"
	"unicode"

	"github.com/shopspring/decimal"
)

var ErrInvalidPayment = errors.New("invalid payment")
var ErrIllegalTransition = errors.New("illegal payment transition")
var ErrInvalidRefund = errors.New("invalid refund")

const (
	StatusCreated           PaymentStatus = "CREATED"
	StatusScreened          PaymentStatus = "SCREENED"
	StatusAuthorized        PaymentStatus = "AUTHORIZED"
	StatusCaptured          PaymentStatus = "CAPTURED"
	StatusSettled           PaymentStatus = "SETTLED"
	StatusPartiallyRefunded PaymentStatus = "PARTIALLY_REFUNDED"
)

func ValidateAmountCurrency(amount decimal.Decimal, currency string) error {
	if amount.Cmp(decimal.Zero) <= 0 || len(currency) != 3 || strings.ToUpper(currency) != currency {
		return ErrInvalidPayment
	}
	for _, r := range currency {
		if !unicode.IsLetter(r) || r > unicode.MaxASCII {
			return ErrInvalidPayment
		}
	}
	return nil
}

func CanTransition(from, to PaymentStatus) bool {
	switch from {
	case StatusCreated:
		return to == StatusScreened || to == StatusFailed
	case StatusPending:
		return to == StatusScreened || to == StatusFailed
	case StatusScreened:
		return to == StatusAuthorized || to == StatusFailed
	case StatusAuthorized:
		return to == StatusCaptured || to == StatusFailed
	case StatusCaptured:
		return to == StatusSettled || to == StatusFailed
	case StatusSettled:
		return to == StatusRefunded || to == StatusPartiallyRefunded
	case StatusPartiallyRefunded:
		// A further positive refund may still leave an outstanding balance.
		return to == StatusPartiallyRefunded || to == StatusRefunded
	}
	return false
}

func Transition(p Payment, to PaymentStatus) (Payment, error) {
	if !CanTransition(p.Status, to) {
		return p, ErrIllegalTransition
	}
	p.Status = to
	return p, nil
}

// ValidateRefund enforces positive cumulative refunds no greater than captured payment value.
func ValidateRefund(paymentAmount, cumulativeRefund, refund decimal.Decimal) error {
	if paymentAmount.Cmp(decimal.Zero) <= 0 || refund.Cmp(decimal.Zero) <= 0 || cumulativeRefund.Cmp(decimal.Zero) < 0 || cumulativeRefund.Add(refund).Cmp(paymentAmount) > 0 {
		return ErrInvalidRefund
	}
	return nil
}
