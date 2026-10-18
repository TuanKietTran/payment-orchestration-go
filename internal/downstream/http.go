package downstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/TuanKietTran/payment-orchestration-go/internal/domain"
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
	retry  *retryPolicy
}

func NewHTTP(base string) (*HTTP, error) {
	u, err := url.Parse(base)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, fmt.Errorf("invalid downstream URL")
	}
	return &HTTP{base: base, client: &http.Client{Timeout: 2 * time.Second}, retry: defaultRetryPolicy()}, nil
}
func (h *HTTP) request(ctx context.Context, method, path string, body any, out any) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	epoch, err := h.retry.acquire()
	if err != nil {
		return err
	}
	failed := false
	defer func() {
		if ctx.Err() != nil {
			h.retry.abandon(epoch)
		} else {
			h.retry.finish(epoch, failed)
		}
	}()
	for attempt := 0; attempt < h.retry.attempts; attempt++ {
		err, failed = h.attempt(ctx, method, path, payload, out)
		if err == nil || !failed {
			return err
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt+1 < h.retry.attempts {
			if err := h.retry.wait(ctx, h.retry.delay(attempt)); err != nil {
				return err
			}
		}
	}
	return err
}

func (h *HTTP) attempt(ctx context.Context, method, path string, payload []byte, out any) (error, bool) {
	req, err := http.NewRequestWithContext(ctx, method, h.base+path, bytes.NewReader(payload))
	if err != nil {
		return err, false
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return err, true
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, (1<<20)+1))
	if err != nil {
		return err, true
	}
	if len(data) > 1<<20 {
		return fmt.Errorf("downstream response exceeds 1 MiB"), false
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("downstream status %d", resp.StatusCode), resp.StatusCode == 408 || resp.StatusCode == 429 || resp.StatusCode >= 500
	}
	if out != nil {
		return json.Unmarshal(data, out), false
	}
	return nil, false
}
func (h *HTTP) Score(ctx context.Context, event, account string, amount decimal.Decimal, currency string) (FraudDecision, error) {
	var out FraudDecision
	if err := domain.ValidateAmountCurrency(amount, currency); err != nil {
		return out, err
	}
	err := h.request(ctx, http.MethodPost, "/v1/score", map[string]string{"event_id": event, "account": account, "amount": amount.StringFixedBank(2), "currency": currency}, &out)
	return out, err
}
func (h *HTTP) Post(ctx context.Context, key, source, dest string, amount decimal.Decimal, currency string) (string, error) {
	if err := domain.ValidateAmountCurrency(amount, currency); err != nil {
		return "", err
	}
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
	if err := domain.ValidateAmountCurrency(amount, currency); err != nil {
		return FraudDecision{}, err
	}
	d := "ALLOW"
	if amount.Cmp(decimal.NewFromInt(20000)) >= 0 {
		d = "BLOCK"
	} else if amount.Cmp(decimal.NewFromInt(6000)) >= 0 {
		d = "REVIEW"
	}
	return FraudDecision{EventID: event, Decision: d}, nil
}

type SimulatedLedger struct {
	mu           sync.Mutex
	Transactions map[string]string
}

func NewSimulatedLedger() *SimulatedLedger {
	return &SimulatedLedger{Transactions: map[string]string{}}
}
func (l *SimulatedLedger) Post(_ context.Context, key, source, dest string, amount decimal.Decimal, currency string) (string, error) {
	if err := domain.ValidateAmountCurrency(amount, currency); err != nil {
		return "", err
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if id, ok := l.Transactions[key]; ok {
		return id, nil
	}
	id := uuid.NewString()
	l.Transactions[key] = id
	return id, nil
}
