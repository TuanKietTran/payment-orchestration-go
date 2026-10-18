package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func TestExactSupportedMoneyBoundaries(t *testing.T) {
	for _, tc := range []struct {
		amount, currency string
		valid            bool
	}{
		{"0.01", "USD", true}, {MaxPaymentAmount, "USD", true}, {"1000000000000.00", "USD", false},
		{"1.001", "USD", false}, {"1.2300", "USD", true}, {"1", "EUR", true}, {"1", "GBP", true},
		{"1", "JPY", false}, {"1", "ABC", false}, {"1e2147483647", "USD", false}, {"1e-2147483647", "USD", false},
		{"0e2147483647", "USD", false}, {"0e-19", "USD", false}, {"9e11", "USD", true}, {"1e-2", "USD", true}, {"1e-3", "USD", false},
	} {
		amount := decimal.RequireFromString(tc.amount)
		if got := ValidateAmountCurrency(amount, tc.currency) == nil; got != tc.valid {
			t.Fatalf("amount=%s currency=%s got=%v", tc.amount, tc.currency, got)
		}
	}
}
func TestRefundMinorUnitConservation(t *testing.T) {
	amount := decimal.New(1000, -2)
	for cents := int64(0); cents < 1000; cents++ {
		already := decimal.New(cents, -2)
		remaining := decimal.New(1000-cents, -2)
		if err := ValidateRefund(amount, already, remaining); err != nil {
			t.Fatal(err)
		}
		if err := ValidateRefund(amount, already, remaining.Add(decimal.New(1, -2))); err == nil {
			t.Fatal("accepted over-refund")
		}
	}
	for _, args := range [][3]string{{"10", "0", "0.001"}, {"10", "0.001", "1"}, {"10.001", "0", "1"}, {"10", "0", "1e2147483647"}, {"10", "1e-2147483647", "1"}} {
		if err := ValidateRefund(decimal.RequireFromString(args[0]), decimal.RequireFromString(args[1]), decimal.RequireFromString(args[2])); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}
func FuzzParseMoneyBounds(f *testing.F) {
	for _, seed := range []string{"1.23", "1.001", MaxPaymentAmount, "1e2147483647", "1e-2147483647", "-1", "NaN"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw string) {
		if len(raw) > 128 {
			t.Skip()
		}
		amount, err := decimal.NewFromString(raw)
		if err != nil {
			return
		}
		if ValidateAmountCurrency(amount, "USD") == nil {
			if amount.Exponent() < -18 || amount.Exponent() > 12 || amount.Sign() <= 0 || !amount.Equal(amount.Truncate(2)) || amount.GreaterThan(maxPaymentAmount) {
				t.Fatal("accepted invalid money")
			}
		}
		_ = ValidateRefund(decimal.NewFromInt(10), decimal.Zero, amount)
	})
}
