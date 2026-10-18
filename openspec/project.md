# Payment orchestration

CQRS payment record. Callers create a pending payment and read it back by id.

## Stack

Go 1.22, Chi, `shopspring/decimal`, Google UUID. Postgres via `jackc/pgx` v5 when configured.

## Public API

- `POST /v1/payments`
- `GET /v1/payments/{id}`
- `POST /v1/payments/{id}/process` screens and posts allowed payments using a recoverable ledger intent.
- `POST /v1/payments/{id}/refunds` reserves and posts bounded partial/full reverse transfers.
- `api/openapi.yaml` is authoritative: create/get/process/refunds are implemented; capture/list/reconciliation remain planned.

## Checkpoints

| Date | Change | Evidence |
| --- | --- | --- |
| 2026-10-18 | production-payments contract publication | `a388318`; create/get contract parsed; later process slice expanded runtime. |
| 2026-10-18 | durable ledger processing intent | `baaa1e2`; `GOTOOLCHAIN=local go test -p 2 ./... -race -count=1 -cover` passed; native embedded Postgres repository 78.9%; weighted non-cmd 68.03%. Saved evidence: control `logs/payments/card4c-gates.log`. |
| 2026-10-18 | bounded downstream resilience | `3ae0248`; `GOTOOLCHAIN=local go test -p 2 ./... -race -count=1 -cover` passed; downstream 86.4%, weighted non-cmd 76.23%. Evidence: control `logs/payments/card4d-gates.log`. |

| Date | Change | Evidence |
| --- | --- | --- |
| 2026-10-17 | Handler tests and Postgres repository | `90eefb7`. `go test ./...`. Embedded Postgres 16 covered create, idempotency, status update, and a missing payment. |
| 2026-10-17 | 1000 requests in one minute, retry, playback, coverage | `internal/playback`. 1000 creates plus one 503 retry each finished in 79ms. Replay returned the original ids and did not add payments. Statement coverage 46.0%. |
| 2026-10-17 | Definition of done and playback storyboard | `openspec/dod.md` and `openspec/gifs/playback-flow.gif`. GIF review: retry stays at the pre-write state, playback shows the unchanged result. |

## Out of scope until implemented

- `CancelPayment`, `RefundPayment`, and `ListPayments` are named in the README and have no HTTP route.
- The repository can update a status. That port is not exposed on the API.

## Definition of done

See `openspec/dod.md`. The playback storyboard is `openspec/gifs/playback-flow.gif`.
