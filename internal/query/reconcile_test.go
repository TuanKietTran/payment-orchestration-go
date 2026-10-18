package query

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
)

type fakeLister struct{ payments []domain.Payment }

func (f fakeLister) List(_ context.Context, _ uuid.UUID, limit int) ([]domain.Payment, error) {
	if len(f.payments) > limit {
		return f.payments[:limit], nil
	}
	return f.payments, nil
}

type fakeReader struct {
	txs map[string]downstream.LedgerTransaction
	err error
}

func (f fakeReader) Transaction(_ context.Context, id string) (downstream.LedgerTransaction, error) {
	if f.err != nil {
		return downstream.LedgerTransaction{}, f.err
	}
	t, ok := f.txs[id]
	if !ok {
		return t, &downstream.StatusError{Code: http.StatusNotFound}
	}
	return t, nil
}

func pay(status domain.PaymentStatus, tx *string) domain.Payment {
	return domain.Payment{ID: uuid.New(), SourceAccount: "s", DestAccount: "d", Amount: decimal.RequireFromString("10.00"), Currency: "USD", Status: status, LedgerTxID: tx}
}

func entries(src, dst, amt, cur string) []downstream.LedgerEntry {
	return []downstream.LedgerEntry{{AccountID: src, Amount: amt, Direction: "DEBIT", Currency: cur}, {AccountID: dst, Amount: amt, Direction: "CREDIT", Currency: cur}}
}

func TestReconcileClassifiesDiscrepancies(t *testing.T) {
	str := func(s string) *string { return &s }
	cases := []struct {
		name string
		p    domain.Payment
		tx   downstream.LedgerTransaction
		want string
		skip bool
	}{
		{"match", pay("SETTLED", str("m")), downstream.LedgerTransaction{Entries: entries("s", "d", "10.00", "USD")}, "", false},
		{"pending skipped", pay("PENDING", nil), downstream.LedgerTransaction{}, "", true},
		{"missing reference", pay("SETTLED", nil), downstream.LedgerTransaction{}, ReconMissingReference, false},
		{"missing ledger", pay("SETTLED", str("absent")), downstream.LedgerTransaction{}, ReconMissingLedger, false},
		{"amount", pay("SETTLED", str("m")), downstream.LedgerTransaction{Entries: entries("s", "d", "11.00", "USD")}, ReconMismatch, false},
		{"currency", pay("SETTLED", str("m")), downstream.LedgerTransaction{Entries: entries("s", "d", "10.00", "EUR")}, ReconMismatch, false},
		{"accounts", pay("SETTLED", str("m")), downstream.LedgerTransaction{Entries: entries("x", "d", "10.00", "USD")}, ReconMismatch, false},
		{"entry count", pay("SETTLED", str("m")), downstream.LedgerTransaction{Entries: entries("s", "d", "10.00", "USD")[:1]}, ReconMismatch, false},
		{"same direction", pay("SETTLED", str("m")), downstream.LedgerTransaction{Entries: []downstream.LedgerEntry{{Direction: "DEBIT"}, {Direction: "DEBIT"}}}, ReconMismatch, false},
		{"bad amount", pay("SETTLED", str("m")), downstream.LedgerTransaction{Entries: entries("s", "d", "zz", "USD")}, ReconMismatch, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader := fakeReader{txs: map[string]downstream.LedgerTransaction{"m": tc.tx}}
			rep, err := NewReconcileHandler(fakeLister{[]domain.Payment{tc.p}}, reader).Handle(context.Background(), uuid.Nil, 10)
			if err != nil {
				t.Fatal(err)
			}
			if tc.skip {
				if rep.Checked != 0 {
					t.Fatalf("checked %d", rep.Checked)
				}
				return
			}
			if tc.want == "" {
				if rep.Matched != 1 || rep.Discrepancies != 0 {
					t.Fatalf("%+v", rep)
				}
				return
			}
			if rep.Discrepancies != 1 || rep.Items[0].Status != tc.want {
				t.Fatalf("%+v", rep)
			}
		})
	}
}

func TestReconcileLedgerErrorAndPaging(t *testing.T) {
	tx := "m"
	p := pay("SETTLED", &tx)
	_, err := NewReconcileHandler(fakeLister{[]domain.Payment{p}}, fakeReader{err: errors.New("down")}).Handle(context.Background(), uuid.Nil, 5)
	if err == nil {
		t.Fatal("expected ledger error")
	}
	l := fakeLister{[]domain.Payment{pay("PENDING", nil), pay("PENDING", nil), pay("PENDING", nil)}}
	rep, err := NewReconcileHandler(l, fakeReader{}).Handle(context.Background(), uuid.Nil, 2)
	if err != nil || rep.NextAfter != l.payments[1].ID.String() {
		t.Fatalf("%+v %v", rep, err)
	}
}
