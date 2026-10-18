# Payment Orchestration (CQRS)

Payment lifecycle service using CQRS.

## Commands
- CreatePayment
- CancelPayment
- RefundPayment

## Queries
- GetPayment
- ListPayments

## Run
```bash
go test ./...
go run ./cmd/server
```

`HTTP_ADDR` defaults to `:8102`. `DATABASE_URL` selects the store. Unset uses the in-memory repository. With Postgres, apply `migrations/001_init.sql` then `migrations/002_processing_outbox.sql` before starting.

`POST /v1/payments/{id}/process` screens the payment. ALLOW records a durable ledger intent before posting and persists `LedgerTxID` on settlement; REVIEW parks at SCREENED; DENY fails without posting. Recovery runs at startup and every second (100 intents per batch). Ledger retries use the same `payment:<idempotency_key>` key, including after an ambiguous downstream commit. `FRAUD_URL` and `LEDGER_URL` select real HTTP adapters; unset values use synthetic, in-memory fixtures, which are not restart durable. Durable recovery requires both a Postgres payment store and a durable idempotent ledger.

HTTP adapters retry transport failures and HTTP 408/429/5xx up to three total attempts, preserving request bytes and replay identity. Each attempt has a two-second client timeout. Backoff is 25ms then 50ms with equal jitter; cancellation interrupts waiting. Three exhausted operations open a per-adapter circuit for five seconds; only one half-open probe is allowed. Other HTTP errors and invalid JSON are final for that call. Responses are capped at 1MiB.

Explicit PSP authorization/capture, fees, refunds and reconciliation are not implemented yet. See `api/openapi.yaml` for callable versus planned routes.
