# Handler tests and Postgres repository

## Why

The payment service had an in-memory CQRS path and a SQL migration, and neither handler tests nor a real Postgres store.

## What

- Unit tests for create, get, idempotency, a missing payment, and status update.
- `PostgresPayment` against `migrations/001_init.sql`.
- `DATABASE_URL` selects Postgres; unset stays in memory.

## Impact

- Specs: `create-payment`, `get-payment` (recorded as the current contract), `persistence` (added).
- No HTTP route added. Cancel, refund, and list stay unimplemented.
