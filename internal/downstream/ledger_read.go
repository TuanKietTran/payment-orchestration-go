package downstream

import (
	"context"
	"fmt"
	"net/http"

	"github.com/google/uuid"
)

// StatusError is a non-2xx downstream response.
type StatusError struct{ Code int }

func (e *StatusError) Error() string { return fmt.Sprintf("downstream status %d", e.Code) }

type LedgerEntry struct {
	AccountID string `json:"account_id"`
	Amount    string `json:"amount"`
	Direction string `json:"direction"`
	Currency  string `json:"currency"`
}

type LedgerTransaction struct {
	TransactionID string        `json:"transaction_id"`
	Entries       []LedgerEntry `json:"entries"`
}

// LedgerReader returns the ledger's own record of a posted transaction.
type LedgerReader interface {
	Transaction(ctx context.Context, id string) (LedgerTransaction, error)
}

func (h *HTTP) Transaction(ctx context.Context, id string) (LedgerTransaction, error) {
	var out LedgerTransaction
	uid, err := uuid.Parse(id)
	if err != nil {
		return out, fmt.Errorf("transaction UUID: %w", err)
	}
	err = h.request(ctx, http.MethodGet, "/v1/transactions/"+uid.String(), nil, &out)
	return out, err
}

func (l *SimulatedLedger) Transaction(_ context.Context, id string) (LedgerTransaction, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.details[id]
	if !ok {
		return LedgerTransaction{}, &StatusError{Code: http.StatusNotFound}
	}
	return t, nil
}
