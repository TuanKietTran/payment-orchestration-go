package downstream

import (
	"context"
	"errors"
	"testing"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

func TestAdaptersRejectMoneyBeforeFormattingOrPosting(t *testing.T) {
	ctx := context.Background()
	client, _ := NewHTTP("http://localhost")
	for _, raw := range []string{"1.001", "1e2147483647", "1e-2147483647"} {
		amount := decimal.RequireFromString(raw)
		if _, err := client.Post(ctx, "key", uuid.NewString(), uuid.NewString(), amount, "USD"); !errors.Is(err, domain.ErrInvalidPayment) {
			t.Fatalf("HTTP posted %s", raw)
		}
		if _, err := client.Score(ctx, "event", "account", amount, "USD"); !errors.Is(err, domain.ErrInvalidPayment) {
			t.Fatalf("HTTP scored %s", raw)
		}
		if _, err := (SimulatedFraud{}).Score(ctx, "event", "account", amount, "USD"); !errors.Is(err, domain.ErrInvalidPayment) {
			t.Fatalf("simulated fraud accepted %s", raw)
		}
		ledger := NewSimulatedLedger()
		if _, err := ledger.Post(ctx, "key", "source", "dest", amount, "USD"); !errors.Is(err, domain.ErrInvalidPayment) || len(ledger.Transactions) != 0 {
			t.Fatalf("simulated ledger accepted %s", raw)
		}
		if _, err := NewSimulatedPSP(nil).Authorize(ctx, "key", amount, "USD"); !errors.Is(err, ErrPSPDeclined) {
			t.Fatalf("PSP accepted %s", raw)
		}
	}
}
