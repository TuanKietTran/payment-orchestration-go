package query

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
	"github.com/TuanKietTran/payment-orchestration-go/internal/downstream"
	"github.com/TuanKietTran/payment-orchestration-go/internal/repository"
)

const (
	ReconMissingReference = "MISSING_REFERENCE"
	ReconMissingLedger    = "MISSING_LEDGER"
	ReconMismatch         = "MISMATCH"
)

type ReconcileItem struct {
	PaymentID string `json:"payment_id"`
	Status    string `json:"status"`
	Detail    string `json:"detail"`
}

type ReconcileReport struct {
	Checked       int             `json:"checked"`
	Matched       int             `json:"matched"`
	Discrepancies int             `json:"discrepancies"`
	Items         []ReconcileItem `json:"items"`
	NextAfter     string          `json:"next_after,omitempty"`
}

// ReconcileHandler compares settled payments with the ledger's own transaction
// record. Only the primary payment posting is checked; refund postings are not.
type ReconcileHandler struct {
	lister repository.PaymentLister
	ledger downstream.LedgerReader
}

func NewReconcileHandler(l repository.PaymentLister, r downstream.LedgerReader) *ReconcileHandler {
	return &ReconcileHandler{lister: l, ledger: r}
}

func (h *ReconcileHandler) Handle(ctx context.Context, after uuid.UUID, limit int) (ReconcileReport, error) {
	report := ReconcileReport{Items: []ReconcileItem{}}
	page, err := h.lister.List(ctx, after, limit+1)
	if err != nil {
		return report, err
	}
	if len(page) > limit {
		page = page[:limit]
		report.NextAfter = page[limit-1].ID.String()
	}
	for _, p := range page {
		item, ok, err := h.check(ctx, p)
		if err != nil {
			return ReconcileReport{Items: []ReconcileItem{}}, err
		}
		if !ok && item.Status == "" {
			continue
		}
		report.Checked++
		if item.Status == "" {
			report.Matched++
			continue
		}
		report.Discrepancies++
		report.Items = append(report.Items, item)
	}
	return report, nil
}

// check returns an empty-status item for a match; ok=false with empty status
// means the payment has no ledger posting to verify.
func (h *ReconcileHandler) check(ctx context.Context, p domain.Payment) (ReconcileItem, bool, error) {
	item := ReconcileItem{PaymentID: p.ID.String()}
	if p.LedgerTxID == nil {
		switch string(p.Status) {
		case "SETTLED", "PARTIALLY_REFUNDED", "REFUNDED":
			item.Status, item.Detail = ReconMissingReference, "settled payment has no ledger transaction reference"
			return item, true, nil
		}
		return item, false, nil
	}
	tx, err := h.ledger.Transaction(ctx, *p.LedgerTxID)
	if err != nil {
		var se *downstream.StatusError
		if errors.As(err, &se) && se.Code == 404 {
			item.Status, item.Detail = ReconMissingLedger, "ledger has no such transaction"
			return item, true, nil
		}
		return item, false, fmt.Errorf("ledger lookup: %w", err)
	}
	if detail := compareLedger(p, tx); detail != "" {
		item.Status, item.Detail = ReconMismatch, detail
		return item, true, nil
	}
	return item, true, nil
}

func compareLedger(p domain.Payment, tx downstream.LedgerTransaction) string {
	if len(tx.Entries) != 2 {
		return fmt.Sprintf("expected 2 ledger entries, got %d", len(tx.Entries))
	}
	var debit, credit *downstream.LedgerEntry
	for i := range tx.Entries {
		switch tx.Entries[i].Direction {
		case "DEBIT":
			debit = &tx.Entries[i]
		case "CREDIT":
			credit = &tx.Entries[i]
		}
	}
	if debit == nil || credit == nil {
		return "ledger entries are not one debit and one credit"
	}
	if debit.AccountID != p.SourceAccount || credit.AccountID != p.DestAccount {
		return "ledger accounts differ from payment"
	}
	for _, e := range []*downstream.LedgerEntry{debit, credit} {
		if e.Currency != p.Currency {
			return "ledger currency differs from payment"
		}
		amt, err := decimal.NewFromString(e.Amount)
		if err != nil || !amt.Equal(p.Amount) {
			return "ledger amount differs from payment"
		}
	}
	return ""
}
