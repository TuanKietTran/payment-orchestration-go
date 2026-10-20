# Payment orchestration

CQRS payment record. Callers create a pending payment and read it back by id.

## Stack

Go 1.22, Chi, `shopspring/decimal`, Google UUID. Postgres via `jackc/pgx` v5 when configured.

## Public API

- `POST /v1/payments`
- `GET /v1/payments/{id}`
- `POST /v1/payments/{id}/process` screens and posts allowed payments using a recoverable ledger intent.
- `POST /v1/payments/{id}/refunds` reserves and posts bounded partial/full reverse transfers.
- `POST /v1/payments/{id}/capture` auto-drives the same screened/authorized/captured/settled flow as process.
- `api/openapi.yaml` is authoritative: create/get/process/capture/refunds are implemented; list/reconciliation remain planned.

## Out of scope until implemented

- `CancelPayment`, `RefundPayment`, and `ListPayments` are named in the README and have no HTTP route.
- The repository can update a status. That port is not exposed on the API.

## Definition of done

See `openspec/dod.md`. The playback storyboard is `openspec/gifs/playback-flow.gif`.
