package downstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type FraudDecision struct {
	EventID  string   `json:"event_id"`
	Score    int      `json:"score"`
	Decision string   `json:"decision"`
	Reasons  []string `json:"reasons"`
}
type Fraud interface {
	Score(context.Context, string, string, decimal.Decimal, string) (FraudDecision, error)
}
type Ledger interface {
	Post(context.Context, string, string, string, decimal.Decimal, string) (string, error)
}
type HTTP struct {
	base   string
	client *http.Client
}

func NewHTTP(base string) (*HTTP, error) {
	u, err := url.Parse(base)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid downstream URL")
	}
	return &HTTP{base: base, client: &http.Client{Timeout: 2 * time.Second}}, nil
}
func (h *HTTP) request(ctx context.Context, method, path string, body any, out any) error {
	var b bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&b).Encode(body); err != nil {
			return err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, h.base+path, &b)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("downstream status %d", resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}
func (h *HTTP) Score(ctx context.Context, event, account string, amount decimal.Decimal, currency string) (FraudDecision, error) {
	var out FraudDecision
	err := h.request(ctx, http.MethodPost, "/v1/score", map[string]string{"event_id": event, "account": account, "amount": amount.StringFixedBank(2), "currency": currency}, &out)
	return out, err
}
func (h *HTTP) Post(ctx context.Context, key, source, dest string, amount decimal.Decimal, currency string) (string, error) {
	sid, e := uuid.Parse(source)
	if e != nil {
		return "", fmt.Errorf("source account UUID: %w", e)
	}
	did, e := uuid.Parse(dest)
	if e != nil {
		return "", fmt.Errorf("destination account UUID: %w", e)
	}
	var out struct {
		TransactionID string `json:"transaction_id"`
	}
	body := map[string]any{"idempotency_key": key, "description": "simulated payment", "entries": []any{map[string]string{"account_id": sid.String(), "amount": amount.StringFixedBank(2), "direction": "DEBIT", "currency": currency}, map[string]string{"account_id": did.String(), "amount": amount.StringFixedBank(2), "direction": "CREDIT", "currency": currency}}}
	err := h.request(ctx, http.MethodPost, "/v1/transactions", body, &out)
	return out.TransactionID, err
}

type SimulatedFraud struct{}

func (SimulatedFraud) Score(_ context.Context, event, account string, amount decimal.Decimal, currency string) (FraudDecision, error) {
	d := "ALLOW"
	if amount.Cmp(decimal.NewFromInt(20000)) >= 0 {
		d = "BLOCK"
	} else if amount.Cmp(decimal.NewFromInt(6000)) >= 0 {
		d = "REVIEW"
	}
	return FraudDecision{EventID: event, Decision: d}, nil
}

type SimulatedLedger struct{ Transactions map[string]string }

func NewSimulatedLedger() *SimulatedLedger {
	return &SimulatedLedger{Transactions: map[string]string{}}
}
func (l *SimulatedLedger) Post(_ context.Context, key, source, dest string, amount decimal.Decimal, currency string) (string, error) {
	if id, ok := l.Transactions[key]; ok {
		return id, nil
	}
	id := uuid.NewString()
	l.Transactions[key] = id
	return id, nil
}
