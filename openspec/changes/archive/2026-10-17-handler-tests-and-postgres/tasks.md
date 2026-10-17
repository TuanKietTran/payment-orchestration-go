# Tasks

## 1. Handler tests

- [x] Create returns a pending payment that get can read
- [x] A reused idempotency key errors with the original id
- [x] An unknown id is `payment not found`
- [x] Status update is visible on the next read, and a missing id fails
- [x] Repository errors propagate from the handlers

## 2. Postgres repository

- [x] Insert and read a payment, including `ledger_tx_id`
- [x] Enforce idempotency on `idempotency_key`
- [x] Update status and reject a missing id
- [x] Select the store from `DATABASE_URL` in `cmd/server`

## 3. Verification

- [x] `go test ./...`
- [x] Embedded Postgres 16: round-trip, idempotency, status, missing id
