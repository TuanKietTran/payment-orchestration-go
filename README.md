# Payment Orchestration (CQRS)

Payment lifecycle service using CQRS.

## Commands
- CreatePayment
- CancelPayment
- RefundPayment

## Queries
- GetPayment
- ListPayments

## Money boundary

USD/EUR/GBP only; positive values up to `999999999999.99`, exact to two minor-unit digits. Excess nonzero fractional precision is rejected, never rounded. Raw decimal exponents outside `[-18,12]` are rejected before rescaling. Numeric JSON compatibility parses directly into decimal, not float. No FX conversion.

## Run
```bash
go test ./...
go run ./cmd/server
```

`HTTP_ADDR` defaults to `:8102`. `DATABASE_URL` selects the store. Unset uses the in-memory repository. With Postgres, apply `migrations/001_init.sql`, `migrations/002_processing_outbox.sql`, `migrations/003_refunds.sql`, then `migrations/004_psp_progress.sql` before starting.

`POST /v1/payments/{id}/process` screens the payment. ALLOW atomically records SCREENED plus a durable intent, then persists synthetic PSP AUTHORIZED/`AuthorizationID` and CAPTURED/`CaptureID` progress before ledger dispatch. Only capture moves cash; authorization is a simulated hold, not another monetary ledger posting. Settlement persists `LedgerTxID`. REVIEW parks at SCREENED without intent (distinguished by `FraudDecision`); DENY or an explicit PSP decline fails without cash posting. `POST /v1/payments/{id}/capture` idempotently auto-drives the same flow as process. Recovery runs at startup and every second (100 intents per batch). Ledger retries use the same `payment:<idempotency_key>` key, including after an ambiguous downstream commit. `FRAUD_URL` and `LEDGER_URL` select real HTTP adapters; unset values use synthetic, in-memory fixtures, which are not restart durable. Durable recovery requires both a Postgres payment store and a durable idempotent ledger.

HTTP adapters retry transport failures and HTTP 408/429/5xx up to three total attempts, preserving request bytes and replay identity. Each attempt has a two-second client timeout. Backoff is 25ms then 50ms with equal jitter; cancellation interrupts waiting. Three exhausted operations open a per-adapter circuit for five seconds; only one half-open probe is allowed. Other HTTP errors and invalid JSON are final for that call. Responses are capped at 1MiB.

`POST /v1/payments/{id}/refunds` accepts `{"idempotency_key":"refund-1","amount":"3.00"}` (or `Idempotency-Key` header) after settlement. Pending reservations count against the original amount, including concurrent calls. The reverse ledger post uses `refund:<refund UUID>`; completion persists its reference and updates `RefundedAmount` plus PARTIALLY_REFUNDED/REFUNDED. Same key/payment/amount replays; changed payload is 409; invalid/over-limit amounts are 400. Pending refunds recover with the payment worker. Refund values support at most two fractional digits; no PSP refund is claimed yet.

The PSP is synthetic: operation references are deterministic and operation keys are stable across retries/restarts. Tests inject transient, before/after-commit and explicit decline faults; the PSP wrapper uses the same bounded retry/breaker policy as HTTP adapters. It is not a real PSP or a durable external account database. Separate authorization journal entries, PSP refunds, and nonzero fees are not implemented yet. `GET /v1/payments` is keyset-paginated (`after`, `limit` 1-200) and `GET /v1/reconciliation` compares settled payments with the ledger's own transaction record (primary posting only; refund postings are not checked). See `api/openapi.yaml` for callable versus planned routes.
