package domain

import (
	"github.com/shopspring/decimal"
	"testing"
)

func TestRefundStateClassification(t *testing.T) {
	for _, status := range []PaymentStatus{StatusPending, StatusCreated, StatusScreened, StatusProcessing, StatusAuthorized, StatusCaptured, StatusFailed, StatusSettled, StatusPartiallyRefunded, StatusRefunded} {
		want := status == StatusSettled || status == StatusPartiallyRefunded || status == StatusRefunded
		if CanRefund(status) != want {
			t.Fatalf("CanRefund(%s)", status)
		}
	}
	amount := decimal.RequireFromString("10.00")
	if got := RefundStatus(decimal.RequireFromString("9.99"), amount); got != StatusPartiallyRefunded {
		t.Fatal(got)
	}
	if got := RefundStatus(amount, amount); got != StatusRefunded {
		t.Fatal(got)
	}
}
