package domain

import (
	"github.com/shopspring/decimal"
	"testing"
)

func TestTransitionLegalAndIllegal(t *testing.T) {
	legal := [][2]PaymentStatus{{StatusCreated, StatusScreened}, {StatusCreated, StatusFailed}, {StatusPending, StatusScreened}, {StatusPending, StatusFailed}, {StatusScreened, StatusAuthorized}, {StatusScreened, StatusFailed}, {StatusAuthorized, StatusCaptured}, {StatusAuthorized, StatusFailed}, {StatusCaptured, StatusSettled}, {StatusCaptured, StatusFailed}, {StatusSettled, StatusPartiallyRefunded}, {StatusSettled, StatusRefunded}, {StatusPartiallyRefunded, StatusRefunded}}
	for _, pair := range legal {
		if !CanTransition(pair[0], pair[1]) {
			t.Errorf("transition %s -> %s rejected", pair[0], pair[1])
		}
	}
	all := []PaymentStatus{StatusCreated, StatusPending, StatusScreened, StatusAuthorized, StatusCaptured, StatusSettled, StatusFailed, StatusRefunded, StatusPartiallyRefunded, "UNKNOWN"}
	for _, from := range all {
		for _, to := range all {
			want := false
			for _, p := range legal {
				if p[0] == from && p[1] == to {
					want = true
				}
			}
			if CanTransition(from, to) != want {
				t.Errorf("%s -> %s: got %v want %v", from, to, CanTransition(from, to), want)
			}
		}
	}
}

func TestTransitionPreservesOnFailure(t *testing.T) {
	p := Payment{Status: StatusCreated}
	got, err := Transition(p, StatusSettled)
	if err != ErrIllegalTransition || got.Status != StatusCreated {
		t.Fatalf("got %+v, %v", got, err)
	}
}

func TestValidateAmountCurrency(t *testing.T) {
	for _, tc := range []struct {
		amount, currency string
		valid            bool
	}{{"1.01", "USD", true}, {"0", "USD", false}, {"-1", "USD", false}, {"1", "usd", false}, {"1", "US", false}, {"1", "US1", false}, {"1", "€SD", false}} {
		err := ValidateAmountCurrency(decimal.RequireFromString(tc.amount), tc.currency)
		if (err == nil) != tc.valid {
			t.Errorf("%s %s: %v", tc.amount, tc.currency, err)
		}
	}
}

func TestValidateRefundBoundary(t *testing.T) {
	one := decimal.NewFromInt(1)
	hundred := decimal.NewFromInt(100)
	if err := ValidateRefund(hundred, decimal.NewFromInt(60), decimal.NewFromInt(40)); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][3]decimal.Decimal{{hundred, decimal.NewFromInt(100), one}, {hundred, decimal.Zero, decimal.Zero}, {hundred, decimal.NewFromInt(-1), one}, {decimal.Zero, decimal.Zero, one}} {
		if ValidateRefund(args[0], args[1], args[2]) == nil {
			t.Errorf("invalid refund accepted: %v", args)
		}
	}
}

func FuzzValidateAmountCurrency(f *testing.F) {
	f.Add("USD")
	f.Add("usd")
	f.Add("U$D")
	f.Fuzz(func(t *testing.T, c string) {
		if len(c) > 100 {
			t.Skip()
		}
		err := ValidateAmountCurrency(decimal.NewFromInt(1), c)
		if err == nil {
			if len(c) != 3 || c != string([]byte(c)) {
				t.Fatalf("accepted invalid currency %q", c)
			}
			for _, r := range c {
				if r < 'A' || r > 'Z' {
					t.Fatalf("accepted non-uppercase ASCII %q", c)
				}
			}
		}
	})
}
